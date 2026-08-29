package entity

import "strings"

// AssistantAllowlist decides which accounts may use the AI writing assistant. It is a static list
// of usernames today because that is all the product needs: one account is enabled, everybody else
// gets the same "not enabled for your account" answer.
//
// It is deliberately a type of its own rather than a bare []string comparison inside a handler,
// because it is the seam the real thing replaces. When access becomes something bought rather than
// something configured, an entitlement - a uid, a tier, and the date the payment it was granted
// for runs out - takes this type's place, and Allows becomes a lookup that can also answer
// "expired". Every caller already asks the question in exactly those terms, so nothing above this
// line has to change when it does.
//
// Matching is on the username rather than on the Google account behind it, since a username is the
// whole of a profile's public identity here and is what an operator configuring the list actually
// knows. Case is folded the same way uniqueness is (see User.UsernameKey), so a list naming
// "edgorman" enables the account holding that name however it was typed.
type AssistantAllowlist struct {
	usernames map[string]bool
}

// NewAssistantAllowlist builds the list, ignoring blanks so a trailing separator in a
// comma-separated environment variable does not enable a profile with no name.
func NewAssistantAllowlist(usernames []string) AssistantAllowlist {
	allowed := make(map[string]bool, len(usernames))
	for _, username := range usernames {
		if key := strings.ToLower(strings.TrimSpace(username)); key != "" {
			allowed[key] = true
		}
	}
	return AssistantAllowlist{usernames: allowed}
}

// Allows reports whether a profile may use the assistant. A profile with no username is never
// allowed, so an empty entry in the list - or an unnamed profile - cannot match by accident.
func (a AssistantAllowlist) Allows(user User) bool {
	key := user.UsernameKey()
	return key != "" && a.usernames[key]
}

// Empty reports whether the assistant is enabled for nobody at all, which is what an unconfigured
// deployment looks like.
func (a AssistantAllowlist) Empty() bool {
	return len(a.usernames) == 0
}
