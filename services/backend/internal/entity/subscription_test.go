package entity

import (
	"testing"
	"time"
)

var subscriptionNow = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

// A subscription grants access up to its expiry and not past it, and the two ways of not having
// one - never bought, and run out - answer identically.
func TestSubscriptionActive(t *testing.T) {
	for _, tc := range []struct {
		name         string
		subscription Subscription
		want         bool
	}{
		{"paid up", Subscription{Until: subscriptionNow.Add(time.Hour)}, true},
		{"never bought", Subscription{}, false},
		{"lapsed", Subscription{Until: subscriptionNow.Add(-time.Hour)}, false},
		{"expiring exactly now", Subscription{Until: subscriptionNow}, false},
		{"customer but no access", Subscription{CustomerID: "cus_1"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.subscription.Active(subscriptionNow); got != tc.want {
				t.Errorf("Active() = %v, want %v", got, tc.want)
			}
		})
	}
}

// What is stored is the expiry or nothing at all, so a cancelled subscription leaves a profile in
// the state one that never paid is in - which is what stops a past date being something a query
// for live subscriptions has to know to exclude.
func TestSubscriptionSubscribedUntil(t *testing.T) {
	until := subscriptionNow.Add(30 * 24 * time.Hour)

	if got := (Subscription{Until: until}).SubscribedUntil(); got == nil || !got.Equal(until) {
		t.Errorf("SubscribedUntil() = %v, want %v", got, until)
	}
	if got := (Subscription{CustomerID: "cus_1"}).SubscribedUntil(); got != nil {
		t.Errorf("SubscribedUntil() = %v, want nil for access that has ended", got)
	}
}

// The entitlement the assistant is gated on follows the stored expiry and nothing else, so a
// subscription written by a payment is immediately what decides it.
func TestSubscriptionGrantsTheAssistant(t *testing.T) {
	entitlement := NewAssistantEntitlement(true)
	user := User{ID: "subscriber"}

	if entitlement.Permission(ActionUpdate, user).Allows(user.ID) {
		t.Fatal("an account that has not paid holds the assistant entitlement")
	}

	user.SubscribedUntil = (Subscription{Until: time.Now().UTC().Add(time.Hour)}).SubscribedUntil()
	if !entitlement.Permission(ActionUpdate, user).Allows(user.ID) {
		t.Error("an account that has just paid does not hold the assistant entitlement")
	}
}
