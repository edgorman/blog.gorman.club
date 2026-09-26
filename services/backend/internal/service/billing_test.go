package service

import (
	"context"
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

// fakePayments is an in-memory repository.Payments. Its zero value is unconfigured, which is what
// every test outside this file gets. Signature checking is the stripe package's to test; here a
// delivery is valid exactly when its header is "valid", and its body is the event type and
// subscription id separated by a space.
type fakePayments struct {
	configured    bool
	subscriptions map[string]repository.Subscription
	fetchErr      error
	checkouts     []repository.CheckoutRequest
}

func (p *fakePayments) Configured() bool { return p.configured }

func (p *fakePayments) CheckoutURL(_ context.Context, req repository.CheckoutRequest) (string, error) {
	p.checkouts = append(p.checkouts, req)
	return "https://checkout.stripe.com/c/1", nil
}

func (p *fakePayments) PortalURL(_ context.Context, customerID, _ string) (string, error) {
	return "https://billing.stripe.com/p/" + customerID, nil
}

func (p *fakePayments) Subscription(_ context.Context, id string) (repository.Subscription, error) {
	if p.fetchErr != nil {
		return repository.Subscription{}, p.fetchErr
	}
	sub, ok := p.subscriptions[id]
	if !ok {
		return repository.Subscription{}, errors.New("no such subscription")
	}
	return sub, nil
}

func (p *fakePayments) VerifyWebhook(payload []byte, signature string, _ time.Time) (repository.WebhookEvent, error) {
	if signature != "valid" {
		return repository.WebhookEvent{}, repository.ErrInvalidSignature
	}
	eventType, id, _ := strings.Cut(string(payload), " ")
	return repository.WebhookEvent{Type: eventType, SubscriptionID: id}, nil
}

func webhook(t *testing.T, s *Service, signature, payload string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", strings.NewReader(payload))
	req.Header.Set("Stripe-Signature", signature)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec.Result().StatusCode
}

var periodEnd = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

func subscriber(until *time.Time) *fakeUserRepository {
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel", SubscribedUntil: until, StripeCustomerID: "cus_1"})
	return users
}

func paymentsWith(status string) *fakePayments {
	return &fakePayments{configured: true, subscriptions: map[string]repository.Subscription{
		"sub_1": {ID: "sub_1", CustomerID: "cus_1", UID: "caller", Status: status, CurrentPeriodEnd: periodEnd},
	}}
}

func TestStripeWebhook_LiveSubscriptionSetsThePeriodEnd(t *testing.T) {
	for _, event := range []string{"customer.subscription.created", "customer.subscription.updated"} {
		for _, status := range []string{"active", "trialing", "past_due"} {
			users := subscriber(nil)
			s := newBillingService(users, paymentsWith(status))

			if got := webhook(t, s, "valid", event+" sub_1"); got != http.StatusOK {
				t.Fatalf("%s/%s: status = %d, want 200", event, status, got)
			}
			stored := users.users["caller"]
			if stored.SubscribedUntil == nil || !stored.SubscribedUntil.Equal(periodEnd) {
				t.Errorf("%s/%s: subscribedUntil = %v, want %v", event, status, stored.SubscribedUntil, periodEnd)
			}
			if stored.StripeCustomerID != "cus_1" {
				t.Errorf("%s/%s: customer = %q, want cus_1", event, status, stored.StripeCustomerID)
			}
		}
	}
}

func TestStripeWebhook_EndedSubscriptionClearsAccess(t *testing.T) {
	for _, status := range []string{"canceled", "unpaid", "incomplete", "incomplete_expired"} {
		users := subscriber(&periodEnd)
		s := newBillingService(users, paymentsWith(status))

		if got := webhook(t, s, "valid", "customer.subscription.deleted sub_1"); got != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", status, got)
		}
		if until := users.users["caller"].SubscribedUntil; until != nil {
			t.Errorf("%s: subscribedUntil = %v, want cleared", status, until)
		}
	}
}

// The event says "updated", but the subscription has since been cancelled: the re-fetch is what is
// written, so a stale event cannot bring access back.
func TestStripeWebhook_StaleEventDoesNotOverwriteNewerState(t *testing.T) {
	users := subscriber(&periodEnd)
	s := newBillingService(users, paymentsWith("canceled"))

	if got := webhook(t, s, "valid", "customer.subscription.updated sub_1"); got != http.StatusOK {
		t.Fatalf("status = %d, want 200", got)
	}
	if until := users.users["caller"].SubscribedUntil; until != nil {
		t.Errorf("subscribedUntil = %v, want cleared by the re-fetched state", until)
	}
}

// An old subscription's late "deleted" must not end access a newer subscription is paying for.
func TestStripeWebhook_EndedSubscriptionKeepsANewerOnesAccess(t *testing.T) {
	newer := periodEnd.AddDate(0, 1, 0)
	users := subscriber(&newer)
	s := newBillingService(users, paymentsWith("canceled"))

	if got := webhook(t, s, "valid", "customer.subscription.deleted sub_1"); got != http.StatusOK {
		t.Fatalf("status = %d, want 200", got)
	}
	if until := users.users["caller"].SubscribedUntil; until == nil || !until.Equal(newer) {
		t.Errorf("subscribedUntil = %v, want %v kept", until, newer)
	}
}

func TestStripeWebhook_IgnoresOtherEvents(t *testing.T) {
	for _, payload := range []string{"invoice.paid in_1", "customer.subscription.updated ", "customer.created cus_1"} {
		users := subscriber(nil)
		s := newBillingService(users, paymentsWith("active"))

		if got := webhook(t, s, "valid", payload); got != http.StatusOK {
			t.Errorf("%q: status = %d, want 200", payload, got)
		}
		if users.subscriptionWrites != 0 {
			t.Errorf("%q: wrote a subscription", payload)
		}
	}
}

func TestStripeWebhook_IgnoresASubscriptionWithNoAccount(t *testing.T) {
	users := newFakeUserRepository()
	payments := paymentsWith("active")
	payments.subscriptions["sub_2"] = repository.Subscription{ID: "sub_2", Status: "active", CurrentPeriodEnd: periodEnd}
	payments.subscriptions["sub_3"] = repository.Subscription{ID: "sub_3", UID: "gone", Status: "active", CurrentPeriodEnd: periodEnd}
	s := newBillingService(users, payments)

	for _, id := range []string{"sub_2", "sub_3"} {
		if got := webhook(t, s, "valid", "customer.subscription.created "+id); got != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", id, got)
		}
	}
	if users.subscriptionWrites != 0 {
		t.Error("wrote a subscription for no account")
	}
}

func TestStripeWebhook_RejectsABadSignature(t *testing.T) {
	users := subscriber(nil)
	s := newBillingService(users, paymentsWith("active"))

	if got := webhook(t, s, "forged", "customer.subscription.created sub_1"); got != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", got)
	}
	if users.subscriptionWrites != 0 {
		t.Error("a forged delivery wrote a subscription")
	}
}

func TestStripeWebhook_FailuresAskStripeToRetry(t *testing.T) {
	users := subscriber(nil)
	users.setSubscriptionErr = errors.New("firestore down")
	if got := webhook(t, newBillingService(users, paymentsWith("active")), "valid", "customer.subscription.created sub_1"); got != http.StatusInternalServerError {
		t.Errorf("failed write: status = %d, want 500", got)
	}

	payments := paymentsWith("active")
	payments.fetchErr = errors.New("stripe down")
	if got := webhook(t, newBillingService(subscriber(nil), payments), "valid", "customer.subscription.created sub_1"); got != http.StatusInternalServerError {
		t.Errorf("failed fetch: status = %d, want 500", got)
	}
}

func TestBillingRoutes_AbsentWhenUnconfigured(t *testing.T) {
	s := newFullService(nil, subscriber(nil), nil, nil, nil, nil)
	for _, path := range []string{"/billing/checkout", "/billing/portal", "/billing/webhook"} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if rec.Result().StatusCode != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, rec.Result().StatusCode)
		}
	}
}

func decodeSessionURL(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.URL
}

func TestCreateCheckout(t *testing.T) {
	payments := paymentsWith("active")
	s := newBillingService(subscriber(nil), payments)

	rec := httptest.NewRecorder()
	s.CreateCheckout(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/checkout", nil), "caller"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Result().StatusCode)
	}
	if got := decodeSessionURL(t, rec); got != "https://checkout.stripe.com/c/1" {
		t.Errorf("url = %q", got)
	}
	want := repository.CheckoutRequest{
		UID:        "caller",
		CustomerID: "cus_1",
		SuccessURL: "https://blog.example/user/calm-smiling-kestrel?checkout=success",
		CancelURL:  "https://blog.example/user/calm-smiling-kestrel",
	}
	if len(payments.checkouts) != 1 || payments.checkouts[0] != want {
		t.Errorf("checkouts = %+v, want [%+v]", payments.checkouts, want)
	}
}

func TestCreateCheckout_RefusesALiveSubscriber(t *testing.T) {
	until := time.Now().Add(time.Hour)
	payments := paymentsWith("active")
	s := newBillingService(subscriber(&until), payments)

	rec := httptest.NewRecorder()
	s.CreateCheckout(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/checkout", nil), "caller"))

	if rec.Result().StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Result().StatusCode)
	}
	if len(payments.checkouts) != 0 {
		t.Error("opened a second checkout for a live subscriber")
	}
}

func TestCreateCheckout_NeedsAProfile(t *testing.T) {
	s := newBillingService(newFakeUserRepository(), paymentsWith("active"))

	rec := httptest.NewRecorder()
	s.CreateCheckout(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/checkout", nil), "caller"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Result().StatusCode)
	}
}

func TestCreatePortal(t *testing.T) {
	s := newBillingService(subscriber(nil), paymentsWith("active"))

	rec := httptest.NewRecorder()
	s.CreatePortal(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/portal", nil), "caller"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Result().StatusCode)
	}
	if got := decodeSessionURL(t, rec); got != "https://billing.stripe.com/p/cus_1" {
		t.Errorf("url = %q", got)
	}
}

func TestCreatePortal_NeedsACustomer(t *testing.T) {
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "caller", Username: "calm-smiling-kestrel"})
	s := newBillingService(users, paymentsWith("active"))

	rec := httptest.NewRecorder()
	s.CreatePortal(rec, withUID(httptest.NewRequest(http.MethodPost, "/billing/portal", nil), "caller"))

	if rec.Result().StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Result().StatusCode)
	}
}
