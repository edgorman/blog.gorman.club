package entity

import "strings"

// AssistantAllowlist decides which accounts may use the AI writing assistant. It is a static list
// of email addresses today because that is all the product needs: one account is enabled,
// everybody else gets the same "not enabled for your account" answer.
//
// It is deliberately a type of its own rather than a bare []string comparison inside a handler,
// because it is the seam the real thing replaces. When access becomes something bought rather than
// something configured, an entitlement - a uid, a tier, and the date the payment it was granted
// for runs out - takes this type's place, and Allows becomes a lookup that can also answer
// "expired". Every caller already asks the question in exactly those terms, so nothing above this
// line has to change when it does.
//
// Matching is on the address the identity provider verified rather than on the username the
// profile holds. A username is the profile's public identity, but it is also freely chosen,
// released when a profile is deleted, and then claimable by anybody - so a list naming one would
// hand the assistant to whoever held that name at the time, not to the person it was written for.
// A verified address names an account, which is what an allowlist is actually about.
type AssistantAllowlist struct {
	emails map[string]bool
}

// NewAssistantAllowlist builds the list, folding case and ignoring blanks so a trailing separator
// in a comma-separated environment variable does not enable an empty address.
func NewAssistantAllowlist(emails []string) AssistantAllowlist {
	allowed := make(map[string]bool, len(emails))
	for _, email := range emails {
		if key := strings.ToLower(strings.TrimSpace(email)); key != "" {
			allowed[key] = true
		}
	}
	return AssistantAllowlist{emails: allowed}
}

// Allows reports whether a caller may use the assistant.
//
// An address the provider did not verify never matches, however exactly it is spelled. That check
// is the whole reason this is safe to key on an address at all: without it, a provider that let an
// account claim an unverified address would let it claim this one.
func (a AssistantAllowlist) Allows(caller Caller) bool {
	if !caller.EmailVerified {
		return false
	}

	key := strings.ToLower(strings.TrimSpace(caller.Email))
	return key != "" && a.emails[key]
}

// Empty reports whether the assistant is enabled for nobody at all, which is what an unconfigured
// deployment looks like.
func (a AssistantAllowlist) Empty() bool {
	return len(a.emails) == 0
}
