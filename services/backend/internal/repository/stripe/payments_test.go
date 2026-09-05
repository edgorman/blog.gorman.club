package stripe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// stripeStub stands in for Stripe's API, recording the form a checkout was created with.
type stripeStub struct {
	*httptest.Server
	form          url.Values
	authorization string
	status        int
	body          string
}

func newStripeStub(t *testing.T) *stripeStub {
	t.Helper()

	stub := &stripeStub{status: http.StatusOK, body: `{"id":"cs_1","url":"https://checkout.stripe.com/c/pay/cs_1"}`}
	stub.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		stub.form = r.PostForm
		stub.authorization = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		_, _ = w.Write([]byte(stub.body))
	}))
	t.Cleanup(stub.Close)
	return stub
}

func TestCheckout_SendsTheBuyerToStripe(t *testing.T) {
	stub := newStripeStub(t)
	p := NewPayments(Config{
		SecretKey:     "sk_test_123",
		WebhookSecret: testWebhookSecret,
		PriceID:       "price_123",
		BaseURL:       stub.URL,
		HTTPClient:    stub.Client(),
	})

	got, err := p.Checkout(context.Background(), repository.CheckoutRequest{
		UserID:     testUID,
		Email:      "author@example.test",
		SuccessURL: "https://blog.test/?subscription=success",
		CancelURL:  "https://blog.test/?subscription=cancelled",
	})
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if want := "https://checkout.stripe.com/c/pay/cs_1"; got != want {
		t.Errorf("Checkout = %q, want %q", got, want)
	}

	if want := "Bearer sk_test_123"; stub.authorization != want {
		t.Errorf("Authorization = %q, want %q", stub.authorization, want)
	}
	for field, want := range map[string]string{
		"mode":                    "subscription",
		"line_items[0][price]":    "price_123",
		"line_items[0][quantity]": "1",
		"success_url":             "https://blog.test/?subscription=success",
		"cancel_url":              "https://blog.test/?subscription=cancelled",
		"customer_email":          "author@example.test",
		// The uid rides on the subscription rather than only on the session, which is what makes
		// a renewal event a year from now still name the account that bought it.
		"subscription_data[metadata][userId]": testUID,
		"client_reference_id":                 testUID,
	} {
		if got := stub.form.Get(field); got != want {
			t.Errorf("form[%q] = %q, want %q", field, got, want)
		}
	}
}

// A checkout with nobody to grant the subscription to takes money for nothing, so it never reaches
// Stripe at all.
func TestCheckout_RefusesWithoutAnAccount(t *testing.T) {
	stub := newStripeStub(t)
	p := NewPayments(Config{
		SecretKey: "sk_test", WebhookSecret: testWebhookSecret, PriceID: "price_123",
		BaseURL: stub.URL, HTTPClient: stub.Client(),
	})

	if _, err := p.Checkout(context.Background(), repository.CheckoutRequest{}); err == nil {
		t.Fatal("Checkout with no user id succeeded")
	}
	if stub.form != nil {
		t.Error("a checkout with no account still reached Stripe")
	}
}

func TestCheckout_ReportsStripesFailure(t *testing.T) {
	stub := newStripeStub(t)
	stub.status = http.StatusBadRequest
	stub.body = `{"error":{"message":"No such price: price_123"}}`
	p := NewPayments(Config{
		SecretKey: "sk_test", WebhookSecret: testWebhookSecret, PriceID: "price_123",
		BaseURL: stub.URL, HTTPClient: stub.Client(),
	})

	_, err := p.Checkout(context.Background(), repository.CheckoutRequest{UserID: testUID})
	if err == nil {
		t.Fatal("Checkout succeeded against a rejecting Stripe")
	}
	// Stripe's own words, so an operator reading the log learns which of their ids is wrong.
	if !strings.Contains(err.Error(), "No such price") {
		t.Errorf("error = %q, want Stripe's message in it", err)
	}
}

func TestBillingPortal_OpensTheProvidersOwnPage(t *testing.T) {
	stub := newStripeStub(t)
	stub.body = `{"id":"bps_1","url":"https://billing.stripe.com/p/session/1"}`
	p := NewPayments(Config{
		SecretKey: "sk_test", WebhookSecret: testWebhookSecret, PriceID: "price_123",
		BaseURL: stub.URL, HTTPClient: stub.Client(),
	})

	got, err := p.BillingPortal(context.Background(), "cus_1", "https://blog.test/")
	if err != nil {
		t.Fatalf("BillingPortal: %v", err)
	}
	if want := "https://billing.stripe.com/p/session/1"; got != want {
		t.Errorf("BillingPortal = %q, want %q", got, want)
	}
	if stub.form.Get("customer") != "cus_1" {
		t.Errorf("form[customer] = %q, want %q", stub.form.Get("customer"), "cus_1")
	}
	if stub.form.Get("return_url") != "https://blog.test/" {
		t.Errorf("form[return_url] = %q, want the caller's frontend", stub.form.Get("return_url"))
	}
}

// An account with no customer at Stripe has nothing to manage, and asking about an empty one would
// only be a slower way of finding that out.
func TestBillingPortal_RefusesWithoutACustomer(t *testing.T) {
	stub := newStripeStub(t)
	p := NewPayments(Config{
		SecretKey: "sk_test", WebhookSecret: testWebhookSecret, PriceID: "price_123",
		BaseURL: stub.URL, HTTPClient: stub.Client(),
	})

	if _, err := p.BillingPortal(context.Background(), "", "https://blog.test/"); err == nil {
		t.Fatal("BillingPortal with no customer succeeded")
	}
	if stub.form != nil {
		t.Error("a portal session with no customer still reached Stripe")
	}
}

// Configured is all-or-nothing on purpose: a deployment that can sell a subscription but cannot be
// told about one takes a buyer's money and grants them nothing.
func TestConfigured(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{"fully configured", Config{SecretKey: "sk", WebhookSecret: "whsec", PriceID: "price"}, true},
		{"nothing configured", Config{}, false},
		{"no api key", Config{WebhookSecret: "whsec", PriceID: "price"}, false},
		{"no webhook secret", Config{SecretKey: "sk", PriceID: "price"}, false},
		{"nothing to sell", Config{SecretKey: "sk", WebhookSecret: "whsec"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewPayments(tc.cfg).Configured(); got != tc.want {
				t.Errorf("Configured() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckout_UnconfiguredRefuses(t *testing.T) {
	p := NewPayments(Config{})

	_, err := p.Checkout(context.Background(), repository.CheckoutRequest{UserID: testUID})
	if !errors.Is(err, repository.ErrPaymentsNotConfigured) {
		t.Fatalf("Checkout error = %v, want ErrPaymentsNotConfigured", err)
	}
}
