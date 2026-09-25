package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	testSecret = "whsec_test"
	testEvent  = `{"type":"customer.subscription.updated","data":{"object":{"id":"sub_1","object":"subscription"}}}`
)

// sign builds a Stripe-Signature header by hand, the way Stripe documents it, rather than through
// anything in this package - so a mistake in VerifyWebhook cannot be mirrored here and pass.
func sign(secret string, at time.Time, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", at.Unix(), payload)
	return fmt.Sprintf("t=%d,v1=%s", at.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

func newTestPayments() *Payments {
	return NewPayments(Config{SecretKey: "sk_test", WebhookSecret: testSecret, PriceID: "price_1"})
}

func TestVerifyWebhook_AcceptsACorrectlySignedDelivery(t *testing.T) {
	now := time.Now()
	event, err := newTestPayments().VerifyWebhook([]byte(testEvent), sign(testSecret, now, testEvent), now)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	want := repository.WebhookEvent{Type: "customer.subscription.updated", SubscriptionID: "sub_1"}
	if event != want {
		t.Errorf("event = %+v, want %+v", event, want)
	}
}

// Stripe sends a v1 per secret while one is being rolled, so any one matching is enough.
func TestVerifyWebhook_AcceptsAnyMatchingSignature(t *testing.T) {
	now := time.Now()
	header := sign("whsec_old", now, testEvent) + "," + sign(testSecret, now, testEvent)[len(fmt.Sprintf("t=%d,", now.Unix())):]
	if _, err := newTestPayments().VerifyWebhook([]byte(testEvent), header, now); err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
}

func TestVerifyWebhook_Rejects(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		payload string
		header  string
	}{
		{"wrong secret", testEvent, sign("whsec_other", now, testEvent)},
		{"tampered body", `{"type":"customer.subscription.updated","data":{"object":{"id":"sub_2","object":"subscription"}}}`, sign(testSecret, now, testEvent)},
		{"missing header", testEvent, ""},
		{"no signature", testEvent, fmt.Sprintf("t=%d", now.Unix())},
		{"no timestamp", testEvent, "v1=" + sign(testSecret, now, testEvent)[len(fmt.Sprintf("t=%d,v1=", now.Unix())):]},
		{"malformed signature", testEvent, fmt.Sprintf("t=%d,v1=zz", now.Unix())},
		{"too old", testEvent, sign(testSecret, now.Add(-webhookTolerance-time.Second), testEvent)},
		{"too far ahead", testEvent, sign(testSecret, now.Add(webhookTolerance+time.Second), testEvent)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTestPayments().VerifyWebhook([]byte(tt.payload), tt.header, now)
			if !errors.Is(err, repository.ErrInvalidSignature) {
				t.Errorf("err = %v, want ErrInvalidSignature", err)
			}
		})
	}
}

// With no secret configured nothing can be verified, including a delivery signed with the empty key.
func TestVerifyWebhook_RejectsWithoutASecret(t *testing.T) {
	now := time.Now()
	payments := NewPayments(Config{})
	if _, err := payments.VerifyWebhook([]byte(testEvent), sign("", now, testEvent), now); !errors.Is(err, repository.ErrInvalidSignature) {
		t.Errorf("err = %v, want ErrInvalidSignature", err)
	}
}

func TestVerifyWebhook_IgnoresOtherObjects(t *testing.T) {
	now := time.Now()
	payload := `{"type":"invoice.paid","data":{"object":{"id":"in_1","object":"invoice"}}}`
	event, err := newTestPayments().VerifyWebhook([]byte(payload), sign(testSecret, now, payload), now)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if event.SubscriptionID != "" {
		t.Errorf("SubscriptionID = %q, want empty for an invoice", event.SubscriptionID)
	}
}

// fakeStripe answers the three endpoints with canned bodies and records what it was sent.
func fakeStripe(t *testing.T, handle func(w http.ResponseWriter, r *http.Request, form url.Values)) *Payments {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk_test" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Stripe-Version"); got != apiVersion {
			t.Errorf("Stripe-Version = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		handle(w, r, form)
	}))
	t.Cleanup(server.Close)
	return NewPayments(Config{SecretKey: "sk_test", WebhookSecret: testSecret, PriceID: "price_1", BaseURL: server.URL})
}

func TestCheckoutURL(t *testing.T) {
	payments := fakeStripe(t, func(w http.ResponseWriter, r *http.Request, form url.Values) {
		if r.URL.Path != "/v1/checkout/sessions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		want := map[string]string{
			"mode":                             "subscription",
			"line_items[0][price]":             "price_1",
			"client_reference_id":              "uid-1",
			"subscription_data[metadata][uid]": "uid-1",
			"customer":                         "cus_1",
			"success_url":                      "https://blog.example/ok",
		}
		for key, value := range want {
			if got := form.Get(key); got != value {
				t.Errorf("%s = %q, want %q", key, got, value)
			}
		}
		_, _ = io.WriteString(w, `{"url":"https://checkout.stripe.com/c/1"}`)
	})

	got, err := payments.CheckoutURL(context.Background(), repository.CheckoutRequest{
		UID: "uid-1", CustomerID: "cus_1", SuccessURL: "https://blog.example/ok", CancelURL: "https://blog.example/",
	})
	if err != nil || got != "https://checkout.stripe.com/c/1" {
		t.Errorf("CheckoutURL = %q, %v", got, err)
	}
}

func TestSubscription_ReadsThePeriodEndFromItems(t *testing.T) {
	payments := fakeStripe(t, func(w http.ResponseWriter, r *http.Request, _ url.Values) {
		if r.URL.Path != "/v1/subscriptions/sub_1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"id":"sub_1","customer":"cus_1","status":"active","metadata":{"uid":"uid-1"},
			"items":{"data":[{"current_period_end":1900000000}]}}`)
	})

	got, err := payments.Subscription(context.Background(), "sub_1")
	if err != nil {
		t.Fatalf("Subscription: %v", err)
	}
	want := repository.Subscription{
		ID: "sub_1", CustomerID: "cus_1", UID: "uid-1", Status: "active", CurrentPeriodEnd: time.Unix(1900000000, 0).UTC(),
	}
	if got != want {
		t.Errorf("Subscription = %+v, want %+v", got, want)
	}
}

func TestSubscription_ProviderError(t *testing.T) {
	payments := fakeStripe(t, func(w http.ResponseWriter, _ *http.Request, _ url.Values) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"message":"No such subscription"}}`)
	})
	if _, err := payments.Subscription(context.Background(), "sub_x"); err == nil {
		t.Error("Subscription succeeded on a 404")
	}
}
