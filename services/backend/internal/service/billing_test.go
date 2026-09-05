package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// newBillingService builds a Service over the profiles a payment writes to and the provider it
// hears from. Nothing else the service holds is involved: buying an entitlement touches no post.
func newBillingService(users repository.UserRepository, payments repository.Payments) *Service {
	return newPaymentsService(nil, users, nil, nil, nil, nil, payments)
}

func TestCreateCheckout_SendsTheCallerToTheProvider(t *testing.T) {
	payments := &fakePayments{configured: true, url: "https://checkout.test/c/pay/cs_1"}
	s := newBillingService(newFakeUserRepository(), payments)

	rec := httptest.NewRecorder()
	req := withCaller(httptest.NewRequest(http.MethodPost, "/billing/checkout", nil),
		entity.Caller{UID: "caller", Email: "author@example.test"})
	s.CreateCheckout(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var body checkoutResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.URL != payments.url {
		t.Errorf("url = %q, want %q", body.URL, payments.url)
	}

	if len(payments.requests) != 1 {
		t.Fatalf("provider saw %d checkouts, want 1", len(payments.requests))
	}
	got := payments.requests[0]
	// The purchase is attached to the verified caller and can be attached to nobody else: there is
	// no id in the request to name somebody to buy for.
	if got.UserID != "caller" {
		t.Errorf("UserID = %q, want %q", got.UserID, "caller")
	}
	if got.Email != "author@example.test" {
		t.Errorf("Email = %q, want the caller's own", got.Email)
	}
	// The buyer comes back to this deployment's frontend, which is the origin CORS admits.
	if !strings.HasPrefix(got.SuccessURL, testOrigin) || !strings.HasPrefix(got.CancelURL, testOrigin) {
		t.Errorf("return URLs = %q and %q, want them under %q", got.SuccessURL, got.CancelURL, testOrigin)
	}
	if got.SuccessURL == got.CancelURL {
		t.Error("a cancelled checkout returns the buyer to the same place a completed one does")
	}
}

// A deployment that cannot take a payment says so rather than failing obscurely, and says it as an
// operator problem: every other route is unaffected.
func TestCreateCheckout_UnconfiguredIsUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		service *Service
	}{
		{"no provider credentials", newBillingService(newFakeUserRepository(), &fakePayments{})},
		{"nowhere to return the buyer", func() *Service {
			s := newBillingService(newFakeUserRepository(), &fakePayments{configured: true})
			s.cfg.AllowedOrigin = ""
			return s
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.service.CreateCheckout(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/checkout", nil), "caller"))

			if rec.Result().StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusServiceUnavailable)
			}
			decodeAPIError(t, rec)
		})
	}
}

// What the provider said about a rejected key or a deleted price is an operator's business: the
// buyer is told the checkout could not start, and nothing else.
func TestCreateCheckout_ProviderFailureIsNotRelayed(t *testing.T) {
	payments := &fakePayments{configured: true, err: errors.New("stripe returned 401: Invalid API Key provided: sk_live_secret")}
	s := newBillingService(newFakeUserRepository(), payments)

	rec := httptest.NewRecorder()
	s.CreateCheckout(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/checkout", nil), "caller"))

	if rec.Result().StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadGateway)
	}
	if body := decodeAPIError(t, rec); strings.Contains(body.Error, "sk_live") {
		t.Errorf("error body = %q, want the provider's own message kept out of it", body.Error)
	}
}

// Cancelling has to be somewhere, or a subscription would be a thing an account could start and
// never stop. It is the provider's own page, reached for the customer on the caller's own profile.
func TestCreateBillingPortalSession_OpensTheCallersOwnBilling(t *testing.T) {
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel", StripeCustomerID: "cus_1"})
	payments := &fakePayments{configured: true, url: "https://billing.test/p/session/1"}
	s := newBillingService(users, payments)

	rec := httptest.NewRecorder()
	s.CreateBillingPortalSession(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/portal", nil), "caller"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var body checkoutResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.URL != payments.url {
		t.Errorf("url = %q, want %q", body.URL, payments.url)
	}
	// The customer is read from the caller's stored profile, which is the whole of the
	// authorization here: there is no id in the request, so there is no way to ask for anybody
	// else's billing.
	if len(payments.portalCustomers) != 1 || payments.portalCustomers[0] != "cus_1" {
		t.Errorf("portal opened for %v, want the caller's own customer", payments.portalCustomers)
	}
	if !strings.HasPrefix(payments.portalReturnURL, testOrigin) {
		t.Errorf("return URL = %q, want it under %q", payments.portalReturnURL, testOrigin)
	}
}

// An account that has never reached a checkout has no customer at the provider, so there is
// nothing to manage - and nothing to ask the provider about on its behalf.
func TestCreateBillingPortalSession_NothingToManage(t *testing.T) {
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel"})
	payments := &fakePayments{configured: true}
	s := newBillingService(users, payments)

	rec := httptest.NewRecorder()
	s.CreateBillingPortalSession(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/portal", nil), "caller"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	if len(payments.portalCustomers) != 0 {
		t.Errorf("the provider was asked about %v, want no call at all", payments.portalCustomers)
	}
}

// The subscription is recorded on the profile and the billing is not, so deleting one while the
// other is live would leave the provider charging for a feature the account can no longer reach.
func TestDeleteUser_RefusesWhileStillPaying(t *testing.T) {
	until := time.Now().UTC().Add(30 * 24 * time.Hour)
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel", SubscribedUntil: &until})
	s := newTestService(nil, users)

	rec := httptest.NewRecorder()
	s.DeleteUser(rec, withUID(httptest.NewRequest(http.MethodDelete, "/users/me", nil), "caller"))

	if rec.Result().StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusConflict)
	}
	if _, ok := users.users["caller"]; !ok {
		t.Error("the profile was deleted anyway")
	}

	// A subscription that has run out is not a reason to keep a profile alive: nothing is being
	// charged for it.
	lapsed := time.Now().UTC().Add(-time.Hour)
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel", SubscribedUntil: &lapsed})

	rec = httptest.NewRecorder()
	s.DeleteUser(rec, withUID(httptest.NewRequest(http.MethodDelete, "/users/me", nil), "caller"))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
}

// The whole point of the webhook: a verified event is what grants paid access, and it grants it to
// the account the event names rather than to whoever sent the request.
func TestStripeWebhook_RecordsTheSubscription(t *testing.T) {
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "subscriber", Username: "calm-smiling-kestrel"})
	until := time.Now().UTC().Add(30 * 24 * time.Hour)
	payments := &fakePayments{
		configured: true,
		event: repository.SubscriptionEvent{
			UserID:       "subscriber",
			Subscription: entity.Subscription{CustomerID: "cus_1", Until: until},
		},
	}
	s := newBillingService(users, payments)

	rec := httptest.NewRecorder()
	body := `{"type":"customer.subscription.created"}`
	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", strings.NewReader(body))
	req.Header.Set(stripeSignatureHeader, "t=1,v1=deadbeef")
	s.HandleStripeWebhook(rec, req)

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	// The signature covers the bytes, so the route has to hand the provider the body exactly as it
	// arrived rather than a re-encoding of it.
	if len(payments.payloads) != 1 || payments.payloads[0] != body {
		t.Errorf("payloads = %q, want the raw body %q", payments.payloads, body)
	}

	stored := users.users["subscriber"]
	if stored.SubscribedUntil == nil || !stored.SubscribedUntil.Equal(until) {
		t.Errorf("SubscribedUntil = %v, want %v", stored.SubscribedUntil, until)
	}
	if stored.StripeCustomerID != "cus_1" {
		t.Errorf("StripeCustomerID = %q, want %q", stored.StripeCustomerID, "cus_1")
	}
	if !stored.Subscribed(time.Now().UTC()) {
		t.Error("the account that just paid is not subscribed")
	}
}

// A subscription that has ended clears the expiry rather than storing a past date, so the account
// is in exactly the state one that never paid is in.
func TestStripeWebhook_RevokesWhenTheSubscriptionEnds(t *testing.T) {
	until := time.Now().UTC().Add(30 * 24 * time.Hour)
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "subscriber", Username: "calm-smiling-kestrel", SubscribedUntil: &until})
	payments := &fakePayments{
		configured: true,
		event: repository.SubscriptionEvent{
			UserID:       "subscriber",
			Subscription: entity.Subscription{CustomerID: "cus_1"},
		},
	}
	s := newBillingService(users, payments)

	rec := httptest.NewRecorder()
	s.HandleStripeWebhook(rec, httptest.NewRequest(http.MethodPost, "/billing/webhook", strings.NewReader("{}")))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if stored := users.users["subscriber"]; stored.SubscribedUntil != nil {
		t.Errorf("SubscribedUntil = %v, want it cleared", stored.SubscribedUntil)
	}
}

// A profile write must not be able to grant paid access, so the only route that can is this one.
func TestPutUser_CannotGrantASubscription(t *testing.T) {
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel"})
	s := newTestService(nil, users)

	// A body naming the field the entitlement reads, spelled as the response reports it.
	rec := httptest.NewRecorder()
	body := `{"bio":"hi","subscribedUntil":"2099-01-01T00:00:00Z","stripeCustomerId":"cus_forged"}`
	s.PutUser(rec, withUID(httptest.NewRequest(http.MethodPut, "/users/me", strings.NewReader(body)), "caller"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if stored := users.users["caller"]; stored.Subscribed(time.Now().UTC()) {
		t.Errorf("an account granted itself a subscription by writing its own profile (until %v)", stored.SubscribedUntil)
	}
}

// How a delivery is answered decides whether Stripe sends it again, so each kind gets the answer
// that makes it stop or come back.
func TestStripeWebhook_AnswersSoStripeRetriesOnlyWhenItShould(t *testing.T) {
	for _, tc := range []struct {
		name     string
		payments *fakePayments
		users    func() *fakeUserRepository
		want     int
	}{
		{
			// Ignoring a delivery and asking for it again forever are not the same thing.
			name:     "nothing to do with this service",
			payments: &fakePayments{configured: true, eventErr: repository.ErrEventIgnored},
			want:     http.StatusNoContent,
		},
		{
			// Never retried into working, and worth a loud log: it is a wrong secret or somebody
			// trying to grant themselves a subscription.
			name:     "unproven delivery",
			payments: &fakePayments{configured: true, eventErr: repository.ErrInvalidSignature},
			want:     http.StatusBadRequest,
		},
		{
			// Redelivery is what makes this endpoint durable enough to be this simple.
			name: "write failed",
			payments: &fakePayments{configured: true, event: repository.SubscriptionEvent{
				UserID: "subscriber", Subscription: entity.Subscription{Until: time.Now().Add(time.Hour)},
			}},
			users: func() *fakeUserRepository {
				users := newFakeUserRepository()
				users.setSubscriptionErr = errors.New("firestore unavailable")
				return users
			},
			want: http.StatusInternalServerError,
		},
		{
			// A payment with no profile to grant it to: retried, so a profile written moments
			// later still gets what it paid for.
			name: "no profile to grant it to",
			payments: &fakePayments{configured: true, event: repository.SubscriptionEvent{
				UserID: "ghost", Subscription: entity.Subscription{Until: time.Now().Add(time.Hour)},
			}},
			want: http.StatusInternalServerError,
		},
		{
			name:     "payments not configured",
			payments: &fakePayments{},
			want:     http.StatusServiceUnavailable,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users := newFakeUserRepository()
			if tc.users != nil {
				users = tc.users()
			}
			s := newBillingService(users, tc.payments)

			rec := httptest.NewRecorder()
			s.HandleStripeWebhook(rec, httptest.NewRequest(http.MethodPost, "/billing/webhook", strings.NewReader("{}")))

			if rec.Result().StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", rec.Result().StatusCode, tc.want)
			}
		})
	}
}

// The webhook is the one route with no credential, so the mux has to leave it that way - and the
// checkout beside it has to stay behind one.
func TestBillingRoutes_Authentication(t *testing.T) {
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel"})
	handler := newBillingService(users, &fakePayments{configured: true, eventErr: repository.ErrEventIgnored}).Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/billing/checkout", nil))
	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated checkout status = %d, want %d", rec.Result().StatusCode, http.StatusUnauthorized)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/billing/webhook", strings.NewReader("{}")))
	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("unauthenticated webhook status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
}
