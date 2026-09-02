package entity

import (
	"testing"
	"time"
)

func verified(email string) Caller {
	return Caller{UID: "uid", Email: email, EmailVerified: true}
}

// entitled is what the routes ask: the permission the entitlement produces, answered for the
// caller it was produced for.
func entitled(e AssistantEntitlement, caller Caller, user User) bool {
	return e.Permission(ActionUpdate, caller, user).Allows(caller.UID)
}

// subscriber is a profile for uid whose paid access runs out in an hour.
func subscriber(uid string) User {
	until := time.Now().UTC().Add(time.Hour)
	return User{ID: uid, SubscribedUntil: &until}
}

func TestAssistantEntitlement_GrantedAddresses(t *testing.T) {
	// Case is folded on both sides, so a granted address entitles the account however the address
	// was spelled in the token.
	entitlement := NewAssistantEntitlement([]string{"ejgorman@gmail.com", "  Someone.Else@example.com  ", ""})

	for _, tt := range []struct {
		email   string
		allowed bool
	}{
		{"ejgorman@gmail.com", true},
		{"EJGorman@Gmail.com", true},
		{"someone.else@example.com", true},
		{"stranger@example.com", false},
		{"ejgorman@gmail.com.evil.example", false},
		{"", false},
	} {
		t.Run(tt.email, func(t *testing.T) {
			if got := entitled(entitlement, verified(tt.email), User{}); got != tt.allowed {
				t.Errorf("entitled(%q) = %v, want %v", tt.email, got, tt.allowed)
			}
		})
	}
}

// An address the provider did not verify never matches a grant. Without this the grant would be
// keyed on something an account could merely claim rather than on something it proved.
func TestAssistantEntitlement_GrantsNeedAVerifiedAddress(t *testing.T) {
	entitlement := NewAssistantEntitlement([]string{"ejgorman@gmail.com"})

	unverified := Caller{UID: "uid", Email: "ejgorman@gmail.com"}
	if entitled(entitlement, unverified, User{}) {
		t.Error("entitled(unverified) = true, want false")
	}
	if !entitled(entitlement, verified("ejgorman@gmail.com"), User{}) {
		t.Error("entitled(verified) = false, want true - the same address, proved")
	}
	// The anonymous caller carries no address at all, so it can never match a grant.
	if entitled(entitlement, Caller{}, User{}) {
		t.Error("entitled(anonymous) = true, want false")
	}
}

// A subscription entitles an account that was never granted anything, which is the whole point of
// the field: access becomes something bought rather than something configured.
func TestAssistantEntitlement_Subscriptions(t *testing.T) {
	entitlement := NewAssistantEntitlement(nil)
	caller := verified("stranger@example.com")

	expired := time.Now().UTC().Add(-time.Minute)
	for _, tt := range []struct {
		name string
		user User
		want bool
	}{
		{"a live subscription", subscriber(caller.UID), true},
		{"a subscription that ran out", User{ID: caller.UID, SubscribedUntil: &expired}, false},
		{"an account that never subscribed", User{ID: caller.UID}, false},
		{"no profile at all", User{}, false},
		// A profile that is not the caller's cannot lend them its subscription, however it was
		// come by: the uid on the profile has to be the uid in the token.
		{"somebody else's subscription", subscriber("another"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := entitled(entitlement, caller, tt.user); got != tt.want {
				t.Errorf("entitled = %v, want %v", got, tt.want)
			}
		})
	}

	// A subscriber does not need a verified address, since the subscription is keyed on the
	// account rather than on anything the address stands in for.
	unverified := Caller{UID: caller.UID, Email: caller.Email}
	if !entitled(entitlement, unverified, subscriber(caller.UID)) {
		t.Error("entitled(subscriber with an unverified address) = false, want true")
	}
}

// A deployment with no model configured is entitled to nobody, however anyone paid: there is
// nothing for an entitlement to buy. That is the zero value, which is what cmd/backend installs.
func TestAssistantEntitlement_DisabledDeployment(t *testing.T) {
	var entitlement AssistantEntitlement

	caller := verified("ejgorman@gmail.com")
	if entitled(entitlement, caller, subscriber(caller.UID)) {
		t.Error("entitled = true on a deployment with no assistant, want false")
	}
	if !entitlement.GrantsNobody() {
		t.Error("GrantsNobody() = false, want true for the zero entitlement")
	}
}

// GrantsNobody describes the deployment's configuration, not who may use the assistant: an
// account that has subscribed is entitled either way, which is why it is not called "empty".
func TestAssistantEntitlement_GrantsNobody(t *testing.T) {
	for _, tt := range []struct {
		name   string
		emails []string
		empty  bool
	}{
		{"unset", nil, true},
		{"blank entries only", []string{"", "  "}, true},
		{"one address", []string{"ejgorman@gmail.com"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entitlement := NewAssistantEntitlement(tt.emails)

			if entitlement.GrantsNobody() != tt.empty {
				t.Errorf("GrantsNobody() = %v, want %v", entitlement.GrantsNobody(), tt.empty)
			}
			if entitled(entitlement, verified("ejgorman@gmail.com"), User{}) == tt.empty {
				t.Errorf("the granted address disagrees with GrantsNobody for %v", tt.emails)
			}
			// Whatever it grants, a subscriber is entitled.
			caller := verified("stranger@example.com")
			if !entitled(entitlement, caller, subscriber(caller.UID)) {
				t.Error("a subscriber is not entitled, want entitled whatever is granted")
			}
		})
	}
}

// Every action on a conversation costs the same entitlement: a transcript is as much a paid
// artifact as the turn that produced it, so reading one back is not cheaper than asking for it.
func TestAssistantEntitlement_EveryActionCostsTheSame(t *testing.T) {
	entitlement := NewAssistantEntitlement([]string{"ejgorman@gmail.com"})

	granted := verified("ejgorman@gmail.com")
	stranger := verified("stranger@example.com")
	for _, action := range []Action{ActionRead, ActionUpdate, ActionDelete} {
		if !entitlement.Permission(action, granted, User{}).Allows(granted.UID) {
			t.Errorf("%q refused an entitled account", action)
		}
		if entitlement.Permission(action, stranger, User{}).Allows(stranger.UID) {
			t.Errorf("%q admitted an account that is not entitled", action)
		}
	}
}
