package entity

import (
	"testing"
	"time"
)

// entitled is what the routes ask: the permission the entitlement produces, answered for the
// account it was produced for.
func entitled(e AssistantEntitlement, user User) bool {
	return e.Permission(ActionUpdate, user).Allows(user.ID)
}

// subscriber is a profile whose paid access runs out in an hour.
func subscriber(uid string) User {
	until := time.Now().UTC().Add(time.Hour)
	return User{ID: uid, SubscribedUntil: &until}
}

// A subscription is the whole of what entitles an account: there is no configured list to be on,
// so access is granted by writing the field and revoked by clearing it.
func TestAssistantEntitlement_Subscriptions(t *testing.T) {
	entitlement := NewAssistantEntitlement(true)

	expired := time.Now().UTC().Add(-time.Minute)
	for _, tt := range []struct {
		name string
		user User
		want bool
	}{
		{"a live subscription", subscriber("uid"), true},
		{"a subscription that ran out", User{ID: "uid", SubscribedUntil: &expired}, false},
		{"an account that never subscribed", User{ID: "uid"}, false},
		{"no profile at all", User{}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := entitled(entitlement, tt.user); got != tt.want {
				t.Errorf("entitled = %v, want %v", got, tt.want)
			}
		})
	}
}

// The permission admits the account it was built for and nobody else, so a subscription cannot be
// spent by a caller who merely knows about it.
func TestAssistantEntitlement_AdmitsOnlyTheSubscriber(t *testing.T) {
	permission := NewAssistantEntitlement(true).Permission(ActionUpdate, subscriber("uid"))

	if !permission.Allows("uid") {
		t.Error("Allows(uid) = false, want the subscriber admitted")
	}
	for _, other := range []string{"another", ""} {
		if permission.Allows(other) {
			t.Errorf("Allows(%q) = true, want only the subscriber", other)
		}
	}
}

// A deployment with no model configured is entitled to nobody, however anyone paid: there is
// nothing for an entitlement to buy. That is the zero value, and what cmd/backend installs when
// no model is set.
func TestAssistantEntitlement_UnavailableDeployment(t *testing.T) {
	for _, entitlement := range []AssistantEntitlement{{}, NewAssistantEntitlement(false)} {
		if entitled(entitlement, subscriber("uid")) {
			t.Error("entitled = true on a deployment with no assistant, want false")
		}
	}
}

// Every action on a conversation costs the same entitlement: a transcript is as much a paid
// artifact as the turn that produced it, so reading one back is not cheaper than asking for it.
func TestAssistantEntitlement_EveryActionCostsTheSame(t *testing.T) {
	entitlement := NewAssistantEntitlement(true)

	paid := subscriber("uid")
	unpaid := User{ID: "another"}
	for _, action := range []Action{ActionRead, ActionUpdate, ActionDelete} {
		if !entitlement.Permission(action, paid).Allows(paid.ID) {
			t.Errorf("%q refused an entitled account", action)
		}
		if entitlement.Permission(action, unpaid).Allows(unpaid.ID) {
			t.Errorf("%q admitted an account that is not entitled", action)
		}
	}
}
