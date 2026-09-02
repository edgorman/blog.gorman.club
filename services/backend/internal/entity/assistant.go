package entity

import (
	"strings"
	"time"
)

// AssistantEntitlement decides which accounts may use the AI writing assistant.
//
// It is a whitelist like any other in the access model (see access.go), with one difference worth
// stating plainly: the list is not a field on a document, it is worked out per request from what
// the account has. An account is entitled while its subscription has not run out
// (User.SubscribedUntil), or unconditionally when this deployment granted its verified address -
// which is how every account is entitled today, since nothing sells a subscription yet. The two
// are the same answer to the caller and to the routes; only the reason differs.
//
// Granting is on the address the identity provider verified rather than on the username the
// profile holds. A username is the profile's public identity, but it is also freely chosen,
// released when a profile is deleted, and then claimable by anybody - so a list naming one would
// hand the assistant to whoever held that name at the time, not to the person it was written for.
// A verified address names an account, which is what a grant is actually about. A subscription
// needs no such care: it is already keyed on the account.
//
// The zero value is a deployment with no assistant at all, entitled to nobody, subscription or
// not - there is nothing for an entitlement to buy when no model is configured.
type AssistantEntitlement struct {
	enabled bool
	granted map[string]bool
}

// NewAssistantEntitlement builds the entitlement for a deployment whose assistant is configured,
// granting it outright to the named addresses. Case is folded and blanks are ignored, so a
// trailing separator in a comma-separated environment variable does not grant an empty address.
func NewAssistantEntitlement(emails []string) AssistantEntitlement {
	granted := make(map[string]bool, len(emails))
	for _, email := range emails {
		if key := strings.ToLower(strings.TrimSpace(email)); key != "" {
			granted[key] = true
		}
	}
	return AssistantEntitlement{enabled: true, granted: granted}
}

// Permission answers who may take an action on the assistant, as a whitelist holding exactly one
// name - the caller's own, when their account is entitled, and nobody at all when it is not.
//
// Answering in the same shape as every other permission is the point: "may I use the assistant" is
// then the same kind of question as "may I read this post", and the routes ask it the same way.
// Where a post carries its readers on the document, this list is computed from an account's
// subscription and this deployment's grants.
//
// user is the caller's own profile, and is ignored unless it is: a profile that is not theirs
// cannot lend them a subscription. A caller with no profile at all is the zero User, which is
// simply not subscribed.
func (e AssistantEntitlement) Permission(action Action, caller Caller, user User) Permission {
	permission := PermissionFor(ResourceAssistant, action)
	if e.entitles(caller, user, time.Now().UTC()) {
		permission.AllowedUserIDs = []string{caller.UID}
	}
	return permission
}

// entitles reports whether the caller's account may spend on the assistant at now.
//
// An address the provider did not verify never matches a grant, however exactly it is spelled.
// That check is the whole reason a grant is safe to key on an address at all: without it, a
// provider that let an account claim an unverified address would let it claim a granted one.
func (e AssistantEntitlement) entitles(caller Caller, user User, now time.Time) bool {
	if !e.enabled || caller.UID == "" {
		return false
	}
	if user.ID == caller.UID && user.Subscribed(now) {
		return true
	}
	if !caller.EmailVerified {
		return false
	}

	key := strings.ToLower(strings.TrimSpace(caller.Email))
	return key != "" && e.granted[key]
}

// GrantsNobody reports whether this deployment grants the assistant to no address outright, which
// is what an unconfigured one looks like. It is not the same as "enabled for nobody": an account
// that has subscribed is entitled either way.
func (e AssistantEntitlement) GrantsNobody() bool {
	return len(e.granted) == 0
}
