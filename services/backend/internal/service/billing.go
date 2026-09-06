package service

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	// maxWebhookBody bounds how much of a delivery is read. The signature covers the whole body,
	// so a truncated one cannot verify - this is not a parser limit but a limit on what an
	// unauthenticated request can make this process allocate, on the one route that has to read a
	// body before it knows who sent it.
	maxWebhookBody = 64 << 10
	// stripeSignatureHeader carries the timestamp and HMACs a delivery is verified against.
	stripeSignatureHeader = "Stripe-Signature"
	// Where Stripe returns the buyer afterward, under this deployment's frontend origin. Neither
	// grants anything: a browser arriving at the first one has been redirected, not verified, and
	// the subscription is written by the webhook below. They differ only so the page can say what
	// happened (see the frontend's landing page).
	checkoutSuccessPath = "/?subscription=success"
	checkoutCancelPath  = "/?subscription=cancelled"
	// Where the provider's billing portal returns somebody afterward. It is the bare frontend
	// because there is nothing to report: whatever they did there arrives as a webhook, so the
	// page they land on will say what is true once it has.
	portalReturnPath = "/"
)

// checkoutResponse is where to send the caller next - the checkout page or the billing portal,
// which are the same answer to a client. It is a URL rather than a redirect because the caller is
// a fetch() from a single-page app: a 303 here would be followed by the browser behind the app's
// back, and the app would be handed Stripe's HTML instead of somewhere to navigate to.
type checkoutResponse struct {
	URL string `json:"url"`
}

// CreateCheckout starts a subscription purchase for the caller and answers with the provider's
// hosted page.
//
// The account is the verified caller's own and cannot be anything else: there is no id in the
// request to name somebody to buy for, in the same way /users/me has none. That is what makes the
// uid this attaches to the subscription trustworthy when it comes back on a webhook months later.
func (s *Service) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	// An operator problem rather than a caller one, so it is a 503 and says so: a deployment with
	// no Stripe credentials (staging before its secrets are filled in, a local backend) still
	// serves everything else.
	if !s.payments.Configured() {
		writeError(w, http.StatusServiceUnavailable, "payments are not configured")
		return
	}
	// Stripe requires somewhere to return the buyer, and this deployment's frontend origin is the
	// only place it could be. It is the same value CORS admits, which is not a coincidence: a
	// deployment that has no frontend allowed to call it has no browser to send anywhere either.
	if s.cfg.AllowedOrigin == "" {
		writeError(w, http.StatusServiceUnavailable, "payments are not configured")
		return
	}

	caller := callerFromContext(r.Context())
	url, err := s.payments.Checkout(r.Context(), repository.CheckoutRequest{
		UserID: caller.UID,
		// Prefills the provider's form. It identifies nothing - access follows the uid above - so
		// an unverified address costs nothing here beyond a buyer correcting it themselves.
		Email:      caller.Email,
		SuccessURL: s.cfg.AllowedOrigin + checkoutSuccessPath,
		CancelURL:  s.cfg.AllowedOrigin + checkoutCancelPath,
	})
	if err != nil {
		// Logged in full and reported in outline: what Stripe said about a rejected key or a
		// deleted price is an operator's business, not a buyer's.
		log.Printf("checkout for %s: %v", caller.UID, err)
		writeError(w, http.StatusBadGateway, "could not start checkout")
		return
	}

	writeJSON(w, http.StatusOK, checkoutResponse{URL: url})
}

// CreateBillingPortalSession sends the caller to the provider's own page for the subscription they
// already have: changing a card, reading invoices, and cancelling.
//
// None of those three is this service's business, and cancelling in particular has to be somewhere
// - a subscription an account could start and not stop would be the worst version of this feature.
// What comes back from a cancellation made there is an ordinary event on the webhook below, so
// nothing else here has to know it happened.
func (s *Service) CreateBillingPortalSession(w http.ResponseWriter, r *http.Request) {
	if !s.payments.Configured() || s.cfg.AllowedOrigin == "" {
		writeError(w, http.StatusServiceUnavailable, "payments are not configured")
		return
	}

	uid := uidFromContext(r.Context())
	user, err := s.users.Get(r.Context(), uid)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// The customer is read from the caller's own stored profile rather than taken from the
	// request, which is the whole of the authorization here: there is no id to send, so there is
	// no way to ask for somebody else's billing.
	if user.StripeCustomerID == "" {
		writeError(w, http.StatusNotFound, "no subscription to manage")
		return
	}

	url, err := s.payments.BillingPortal(r.Context(), user.StripeCustomerID, s.cfg.AllowedOrigin+portalReturnPath)
	if err != nil {
		log.Printf("billing portal for %s: %v", uid, err)
		writeError(w, http.StatusBadGateway, "could not open the billing portal")
		return
	}

	writeJSON(w, http.StatusOK, checkoutResponse{URL: url})
}

// HandleStripeWebhook records what the payment provider says about an account's paid access.
//
// This is the one route with no credential and no rate limit of its own, and the one write in the
// service that a caller cannot reach: it is authenticated by the signature over its body instead
// (see the stripe package), which is checked before the body is parsed. Nothing about the request
// beyond that signature is trusted - in particular the uid it acts on comes off the signed
// subscription rather than from anything the request path or headers claim.
//
// What it answers matters as much as what it does, because Stripe retries a delivery it did not
// get a 2xx for, for days:
//
//   - A delivery this service has nothing to do with is a success. Ignoring it and asking for it
//     again forever are not the same thing.
//   - A bad signature is a 400 and is never retried into working.
//   - A failed write is a 500, so it is redelivered. That is the whole reason this endpoint can
//     afford to be as simple as it is: the retry is the durability.
func (s *Service) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.payments.Configured() {
		writeError(w, http.StatusServiceUnavailable, "payments are not configured")
		return
	}

	// The raw bytes, since the signature covers them: a body decoded and re-encoded would not
	// verify even when it is the same JSON.
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read request body")
		return
	}

	event, err := s.payments.DecodeEvent(payload, r.Header.Get(stripeSignatureHeader))
	switch {
	case errors.Is(err, repository.ErrEventIgnored):
		// Most deliveries land here, and the log line is the point: it is how an operator sees
		// that the endpoint is subscribed to more than it needs, or that somebody created a
		// subscription by hand that will never grant anything.
		log.Printf("stripe webhook: %v", err)
		w.WriteHeader(http.StatusNoContent)
		return
	case errors.Is(err, repository.ErrInvalidSignature):
		// Worth logging loudly: on this route it is either a misconfigured secret or somebody
		// trying to grant themselves a subscription.
		log.Printf("stripe webhook: rejected delivery: %v", err)
		writeError(w, http.StatusBadRequest, "invalid signature")
		return
	case err != nil:
		log.Printf("stripe webhook: %v", err)
		writeError(w, http.StatusBadRequest, "could not decode event")
		return
	}

	if err := s.users.SetSubscription(r.Context(), event.UserID, event.Subscription); err != nil {
		// Including the not-found case, which is a paid subscription with no profile to grant it
		// to - a deleted account, or one that paid before its profile was written. A 500 has
		// Stripe redeliver it, which is what gives the second of those time to resolve itself and
		// leaves the first visible in the logs rather than silently swallowed.
		log.Printf("stripe webhook: record subscription for %s: %v", event.UserID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
