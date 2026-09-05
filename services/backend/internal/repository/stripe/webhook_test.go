package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	testWebhookSecret = "whsec_test"
	testUID           = "google-sub-1"
)

// testNow is the clock every signature below is stamped against, so a test can hold a fixed
// timestamp and still exercise the tolerance window.
var testNow = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

func testPayments() *Payments {
	return NewPayments(Config{
		SecretKey:     "sk_test",
		WebhookSecret: testWebhookSecret,
		PriceID:       "price_test",
		Now:           func() time.Time { return testNow },
	})
}

// sign builds the header Stripe would send for a payload at a given time, which is the only way to
// exercise verification without checking in a captured delivery.
func sign(payload string, at time.Time, secret string) string {
	timestamp := fmt.Sprintf("%d", at.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + payload))
	return fmt.Sprintf("t=%s,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))
}

// subscriptionEvent is a delivery in the shape Stripe sends, with the period end on the item -
// where current API versions carry it.
func subscriptionEvent(eventType, status string, periodEnd time.Time, uid string) string {
	return fmt.Sprintf(`{
		"id": "evt_1",
		"type": %q,
		"data": {"object": {
			"id": "sub_1",
			"status": %q,
			"customer": "cus_1",
			"metadata": {"userId": %q},
			"items": {"data": [{"current_period_end": %d}]}
		}}
	}`, eventType, status, uid, periodEnd.Unix())
}

func TestDecodeEvent_GrantsUntilPeriodEnd(t *testing.T) {
	p := testPayments()
	until := testNow.Add(30 * 24 * time.Hour)
	payload := subscriptionEvent(eventSubscriptionCreated, "active", until, testUID)

	event, err := p.DecodeEvent([]byte(payload), sign(payload, testNow, testWebhookSecret))
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}

	if event.UserID != testUID {
		t.Errorf("UserID = %q, want %q", event.UserID, testUID)
	}
	if event.CustomerID != "cus_1" {
		t.Errorf("CustomerID = %q, want %q", event.CustomerID, "cus_1")
	}
	if !event.Until.Equal(until.Truncate(time.Second)) {
		t.Errorf("Until = %v, want %v", event.Until, until)
	}
	if !event.Active(testNow) {
		t.Error("subscription is not active at the time it was granted")
	}
}

// Older API versions carry the period end on the subscription itself. A webhook endpoint's version
// is a setting on the Stripe account rather than something this code sends, so both shapes have to
// read - and reading neither would grant every subscriber nothing.
func TestDecodeEvent_ReadsLegacyPeriodEnd(t *testing.T) {
	p := testPayments()
	until := testNow.Add(24 * time.Hour)
	payload := fmt.Sprintf(`{"id":"evt_1","type":%q,"data":{"object":{
		"id":"sub_1","status":"active","customer":"cus_1",
		"metadata":{"userId":%q},"current_period_end":%d}}}`,
		eventSubscriptionUpdated, testUID, until.Unix())

	event, err := p.DecodeEvent([]byte(payload), sign(payload, testNow, testWebhookSecret))
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	if !event.Until.Equal(until.Truncate(time.Second)) {
		t.Errorf("Until = %v, want %v", event.Until, until)
	}
}

// Every way a subscription stops granting access ends up as the same stored state: no expiry at
// all. A status outside active/trialing grants nothing even though the period it was paid for has
// not run out - the conservative half of the pair, since the alternative is serving a paid feature
// on a payment that has not cleared.
func TestDecodeEvent_RevokesWhenNotActive(t *testing.T) {
	future := testNow.Add(30 * 24 * time.Hour)
	for _, tc := range []struct {
		name      string
		eventType string
		status    string
	}{
		{"deleted", eventSubscriptionDeleted, "canceled"},
		{"cancelled but still in period", eventSubscriptionUpdated, "canceled"},
		{"payment failing", eventSubscriptionUpdated, "past_due"},
		{"never completed", eventSubscriptionUpdated, "incomplete"},
		{"unpaid", eventSubscriptionUpdated, "unpaid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPayments()
			payload := subscriptionEvent(tc.eventType, tc.status, future, testUID)

			event, err := p.DecodeEvent([]byte(payload), sign(payload, testNow, testWebhookSecret))
			if err != nil {
				t.Fatalf("DecodeEvent: %v", err)
			}
			if !event.Until.IsZero() {
				t.Errorf("Until = %v, want the zero time", event.Until)
			}
			if event.Active(testNow) {
				t.Error("a subscription that is not being paid for still grants access")
			}
			// The customer is still reported, so an account that stops paying keeps the id a
			// later billing flow reaches it by.
			if event.CustomerID != "cus_1" {
				t.Errorf("CustomerID = %q, want it kept", event.CustomerID)
			}
		})
	}
}

// A trial is paid access that has not been billed yet, and is treated as paid: the account is
// inside a period Stripe says runs until a date, which is exactly what is stored.
func TestDecodeEvent_GrantsTrial(t *testing.T) {
	p := testPayments()
	until := testNow.Add(14 * 24 * time.Hour)
	payload := subscriptionEvent(eventSubscriptionUpdated, "trialing", until, testUID)

	event, err := p.DecodeEvent([]byte(payload), sign(payload, testNow, testWebhookSecret))
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	if !event.Active(testNow) {
		t.Error("a trialing subscription grants nothing")
	}
}

func TestDecodeEvent_IgnoresUnrelatedDeliveries(t *testing.T) {
	until := testNow.Add(time.Hour)
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{
			// A webhook endpoint is told about far more than it asked for.
			name:    "another event type",
			payload: `{"id":"evt_1","type":"invoice.paid","data":{"object":{}}}`,
		},
		{
			// A subscription created by hand in the dashboard names no account, and there is
			// nothing to guess from: access follows the uid, never the customer's address.
			name:    "no account attached",
			payload: subscriptionEvent(eventSubscriptionCreated, "active", until, ""),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPayments()

			_, err := p.DecodeEvent([]byte(tc.payload), sign(tc.payload, testNow, testWebhookSecret))
			if !errors.Is(err, repository.ErrEventIgnored) {
				t.Fatalf("DecodeEvent error = %v, want ErrEventIgnored", err)
			}
		})
	}
}

// The whole point of this endpoint's signature: a body nobody can prove Stripe sent is a stranger
// asserting that somebody paid.
func TestDecodeEvent_RejectsUnprovenDeliveries(t *testing.T) {
	until := testNow.Add(30 * 24 * time.Hour)
	payload := subscriptionEvent(eventSubscriptionCreated, "active", until, testUID)

	for _, tc := range []struct {
		name      string
		payload   string
		signature string
	}{
		{"no signature", payload, ""},
		{"malformed header", payload, "not-a-signature"},
		{"signed with another secret", payload, sign(payload, testNow, "whsec_someone_else")},
		{
			// The signature covers the bytes, so a body edited after signing does not verify -
			// which is what stops a genuine delivery being turned into a longer subscription.
			name:      "body edited after signing",
			payload:   strings.Replace(payload, testUID, "somebody-else", 1),
			signature: sign(payload, testNow, testWebhookSecret),
		},
		{
			// Without the tolerance window one captured "you have paid" body would be a permanent
			// free subscription for whoever kept a copy of it.
			name:      "replayed later",
			payload:   payload,
			signature: sign(payload, testNow.Add(-signatureTolerance-time.Minute), testWebhookSecret),
		},
		{
			name:      "stamped in the future",
			payload:   payload,
			signature: sign(payload, testNow.Add(signatureTolerance+time.Minute), testWebhookSecret),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPayments()

			_, err := p.DecodeEvent([]byte(tc.payload), tc.signature)
			if !errors.Is(err, repository.ErrInvalidSignature) {
				t.Fatalf("DecodeEvent error = %v, want ErrInvalidSignature", err)
			}
		})
	}
}

// Stripe signs a delivery with both the old and the new secret while a signing secret is being
// rolled, so any one of the signatures matching is the delivery being genuine.
func TestDecodeEvent_AcceptsOneOfSeveralSignatures(t *testing.T) {
	p := testPayments()
	until := testNow.Add(30 * 24 * time.Hour)
	payload := subscriptionEvent(eventSubscriptionCreated, "active", until, testUID)

	// t=...,v1=<previous secret>,v1=<current secret>, as a rollover delivery arrives.
	header := sign(payload, testNow, "whsec_previous") + "," +
		strings.TrimPrefix(sign(payload, testNow, testWebhookSecret), fmt.Sprintf("t=%d,", testNow.Unix()))

	if _, err := p.DecodeEvent([]byte(payload), header); err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
}

// A deployment with no webhook secret cannot prove anything about a delivery, so it refuses every
// one rather than trusting them all.
func TestDecodeEvent_UnconfiguredRefuses(t *testing.T) {
	p := NewPayments(Config{SecretKey: "sk_test", PriceID: "price_test"})
	payload := subscriptionEvent(eventSubscriptionCreated, "active", testNow.Add(time.Hour), testUID)

	_, err := p.DecodeEvent([]byte(payload), sign(payload, testNow, testWebhookSecret))
	if !errors.Is(err, repository.ErrPaymentsNotConfigured) {
		t.Fatalf("DecodeEvent error = %v, want ErrPaymentsNotConfigured", err)
	}
}
