package entity

import "time"

// Subscription is the state of one account's paid access, as the payment provider last reported
// it. It is the whole of what a payment buys here: there is one tier, so what is bought is time
// rather than a plan, and nothing downstream asks which product was paid for - only whether the
// account may spend (see AssistantEntitlement).
//
// It is deliberately not a record of the billing relationship: there are no invoices, no plan id,
// and no cancellation date, because none of them decide anything this service does. What the
// provider knows about the money stays with the provider; what is stored here is the answer to
// "until when".
type Subscription struct {
	// CustomerID is the provider's own id for the account, kept so a later "manage billing" flow
	// can address the customer that was created for it without having to search for one. It is
	// never reported to any caller.
	CustomerID string
	// Until is when paid access runs out. The zero time is access that has ended - a cancelled
	// subscription and one that was never bought are the same state, deliberately, for the same
	// reason User.Subscribed treats them alike: "lapsed" is a billing question rather than an
	// access one.
	Until time.Time
}

// Active reports whether the subscription grants access at now.
func (s Subscription) Active(now time.Time) bool {
	return !s.Until.IsZero() && now.Before(s.Until)
}

// SubscribedUntil is the value to store on a profile: the expiry as a pointer, and nil for access
// that has ended, so an account that has stopped paying is stored the same way as one that never
// started (see the firestore user document, where the field is omitted entirely).
func (s Subscription) SubscribedUntil() *time.Time {
	if s.Until.IsZero() {
		return nil
	}
	until := s.Until.UTC()
	return &until
}
