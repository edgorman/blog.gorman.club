package service

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	blogv1 "github.com/edgorman/blog.gorman.club/services/backend/internal/gen/blog/v1"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// maxWebhookBytes caps a webhook body. The route is public, so it is the one place a body is read
// before anything has vouched for the sender; a subscription event is a few kilobytes.
const maxWebhookBytes = 256 << 10

// subscriptionEvents are the webhook events acted on. Between them they cover buying, renewing,
// lapsing and cancelling, since each is a change to the subscription's status or period end.
var subscriptionEvents = map[string]bool{
	"customer.subscription.created": true,
	"customer.subscription.updated": true,
	"customer.subscription.deleted": true,
}

// billingEnabled reports whether this deployment sells the subscription. It needs somewhere to
// send the browser back to as well as the keys, so a deployment with no frontend origin has none.
func (s *Service) billingEnabled() bool {
	return s.payments.Configured() && s.cfg.AllowedOrigin != ""
}

// profileURL is the caller's own profile on the frontend, which is where checkout and the portal
// send the browser back to - it is where the subscription is shown.
func (s *Service) profileURL(user entity.User) string {
	return s.cfg.AllowedOrigin + "/user/" + url.PathEscape(user.Username)
}

// billingCaller loads the caller's own profile for a billing route, answering the request itself
// and returning false when it cannot. A profile is required because the webhook writes the
// subscription onto one, and the return URL is addressed by its username.
func (s *Service) billingCaller(w http.ResponseWriter, r *http.Request) (entity.User, bool) {
	uid := uidFromContext(r.Context())
	permission := entity.PermissionFor(entity.ResourceBilling, entity.ActionCreate)
	permission.OwnerID = uid
	if !requirePermission(w, r, permission) {
		return entity.User{}, false
	}

	user, err := s.users.Get(r.Context(), uid)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return entity.User{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return entity.User{}, false
	}
	return user, true
}

// CreateCheckout opens a Stripe Checkout for the caller's own account. Returning from it proves
// nothing - access is granted by the webhook alone - so the success URL only tells the frontend to
// expect the subscription shortly.
func (s *Service) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	user, ok := s.billingCaller(w, r)
	if !ok {
		return
	}
	// A second subscription would bill twice for the same access; a live one is managed in the
	// portal instead.
	if user.Subscribed(time.Now()) {
		writeError(w, http.StatusConflict, "already subscribed")
		return
	}

	checkout, err := s.payments.CheckoutURL(r.Context(), repository.CheckoutRequest{
		UID:        user.ID,
		CustomerID: user.StripeCustomerID,
		SuccessURL: s.profileURL(user) + "?checkout=success",
		CancelURL:  s.profileURL(user),
	})
	if err != nil {
		log.Printf("billing: checkout for %s: %v", user.ID, err)
		writeError(w, http.StatusBadGateway, "payment provider unavailable")
		return
	}
	writeProto(w, http.StatusOK, &blogv1.BillingSessionResponse{Url: checkout})
}

// CreatePortal opens the Stripe Customer Portal for the caller's own account, which is where a
// subscription is cancelled or its card changed.
func (s *Service) CreatePortal(w http.ResponseWriter, r *http.Request) {
	user, ok := s.billingCaller(w, r)
	if !ok {
		return
	}
	if user.StripeCustomerID == "" {
		writeError(w, http.StatusConflict, "no subscription to manage")
		return
	}

	portal, err := s.payments.PortalURL(r.Context(), user.StripeCustomerID, s.profileURL(user))
	if err != nil {
		log.Printf("billing: portal for %s: %v", user.ID, err)
		writeError(w, http.StatusBadGateway, "payment provider unavailable")
		return
	}
	writeProto(w, http.StatusOK, &blogv1.BillingSessionResponse{Url: portal})
}

// StripeWebhook is the only thing that grants access, and the only route with no credential: it is
// authenticated by the Stripe-Signature HMAC over the raw body, checked before the body is parsed.
//
// The status tells Stripe whether to retry: a delivery that was handled or deliberately ignored is
// 2xx, a bad signature 400 (retrying cannot fix it), and a failed read or write 500.
func (s *Service) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	event, err := s.payments.VerifyWebhook(payload, r.Header.Get("Stripe-Signature"), time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid signature")
		return
	}
	if !subscriptionEvents[event.Type] || event.SubscriptionID == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// The subscription is fetched rather than read off the event, because Stripe delivers events out
	// of order and retries them: a stale "updated" arriving after a "deleted" must not bring a
	// cancelled subscription back. Whatever the event, what is written is the state as it is now.
	//
	// Fetch and write happen under one lock, so of two deliveries racing (checkout sends "created"
	// and "updated" milliseconds apart) the one that fetched later also writes later.
	// ponytail: per-process lock, like the rate limiter's buckets; a Firestore-held lease if the
	// service ever runs more than one instance.
	s.webhookMu.Lock()
	defer s.webhookMu.Unlock()

	sub, err := s.payments.Subscription(r.Context(), event.SubscriptionID)
	if err != nil {
		log.Printf("billing: fetch subscription %s: %v", event.SubscriptionID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// A subscription not made through this service's checkout names no account.
	if sub.UID == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	user, err := s.users.Get(r.Context(), sub.UID)
	if errors.Is(err, repository.ErrNotFound) {
		// The account is gone; there is nothing to grant, and retrying will not bring it back.
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	err = s.users.SetSubscription(r.Context(), user.ID, sub.CustomerID, subscribedUntil(user, sub))
	if errors.Is(err, repository.ErrNotFound) {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		log.Printf("billing: write subscription for %s: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// subscribedUntil is what a subscription's current state makes the account's paid access.
//
// A live subscription runs to the end of its current period: active, in a trial, or past_due - a
// failed renewal Stripe is still retrying, which it turns into canceled or unpaid if every retry
// fails. Counting past_due as live keeps the portal (where the card is fixed) in reach, and keeps
// a second checkout and an account deletion refused while Stripe may still charge. One cancelled
// at period end stays active until then, so access ends exactly when the period does. Anything else
// - canceled, unpaid, incomplete - ends access, except that an ended subscription does
// not clear access running later than its own period: that came from a newer subscription, and an
// old one's late or retried event must not take it away.
func subscribedUntil(user entity.User, sub repository.Subscription) *time.Time {
	if sub.Status == "active" || sub.Status == "trialing" || sub.Status == "past_due" {
		end := sub.CurrentPeriodEnd
		return &end
	}
	if user.SubscribedUntil != nil && user.SubscribedUntil.After(sub.CurrentPeriodEnd) {
		return user.SubscribedUntil
	}
	return nil
}
