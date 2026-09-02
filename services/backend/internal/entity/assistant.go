package entity

import "time"

// AssistantEntitlement decides which accounts may use the AI writing assistant.
//
// It is a whitelist like any other in the access model (see access.go), with one difference worth
// stating plainly: the list is not a field on a document, it is worked out per request from what
// the account has. An account is entitled while its subscription has not run out
// (User.SubscribedUntil) and no other way - there is no configured list of addresses to be on,
// so granting access is writing that field and revoking it is clearing it, neither of which
// needs a deployment.
//
// The zero value is a deployment with no assistant at all, entitled to nobody however anyone paid:
// there is nothing for an entitlement to buy when no model is configured.
type AssistantEntitlement struct {
	available bool
}

// NewAssistantEntitlement builds the entitlement for a deployment whose assistant is configured or
// not (see repository.Assistant.Configured).
func NewAssistantEntitlement(available bool) AssistantEntitlement {
	return AssistantEntitlement{available: available}
}

// Permission answers who may take an action on the assistant, as a whitelist holding exactly one
// name - the account's own, when it is entitled, and nobody at all when it is not.
//
// Answering in the same shape as every other permission is the point: "may I use the assistant" is
// then the same kind of question as "may I read this post", and the routes ask it the same way.
// Where a post carries its readers on the document, this list is computed from what the account
// has paid for.
//
// user is the caller's own profile, looked up by the uid in their verified token, so the name this
// admits is the one that asked. A caller with no profile is the zero User, which is simply not
// subscribed.
func (e AssistantEntitlement) Permission(action Action, user User) Permission {
	permission := PermissionFor(ResourceAssistant, action)
	if e.entitles(user, time.Now().UTC()) {
		permission.AllowedUserIDs = []string{user.ID}
	}
	return permission
}

// entitles reports whether the account may spend on the assistant at now.
func (e AssistantEntitlement) entitles(user User, now time.Time) bool {
	return e.available && user.ID != "" && user.Subscribed(now)
}
