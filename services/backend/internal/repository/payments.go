package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// Payments is the payment provider a subscription is bought through. It is narrow on purpose, and
// for the same reason Assistant is: everything that makes a provider a provider - its API shape,
// its webhook signing scheme, the vocabulary of events it emits - lives behind this interface, so
// swapping one for another means adding a folder under repository/ and changing one line in
// cmd/backend.
//
// There are exactly two things this service asks of a provider. Send an account somewhere it can
// pay, and tell me what an account has paid for. Nothing else about the billing relationship -
// invoices, proration, the plan's name - reaches this codebase at all, because nothing here
// decides anything with it (see entity.Subscription).
//
// Nothing behind this interface writes either: DecodeEvent verifies and translates, and the
// service is what stores the result. A provider cannot reach Firestore even in principle, which
// matters more here than it does for the assistant - this is the one input that grants access.
type Payments interface {
	// Configured reports whether this deployment can take a payment at all. A deployment without
	// credentials serves every other route and refuses only these, in the same way one with no
	// model configured refuses only the assistant.
	Configured() bool
	// Checkout starts a purchase and returns the URL to send the buyer to. It returns
	// ErrPaymentsNotConfigured if the deployment cannot take a payment.
	Checkout(ctx context.Context, req CheckoutRequest) (string, error)
	// BillingPortal returns the URL of the provider's own page for managing an existing
	// subscription: changing a card, reading invoices, and cancelling.
	//
	// It exists rather than this service growing those three things because none of them is this
	// service's business - and cancelling in particular must be somewhere, or a subscription would
	// be a thing an account could start and never stop. What comes back from a cancellation is an
	// ordinary event on DecodeEvent below, so the one path that records access records it.
	//
	// customerID is the provider's id for the account, stored the first time an event named one
	// (entity.Subscription.CustomerID). An account that has never reached a checkout has none, and
	// has nothing to manage.
	BillingPortal(ctx context.Context, customerID, returnURL string) (string, error)
	// DecodeEvent verifies a webhook delivery against the provider's signature and translates it
	// into what it says about one account's paid access.
	//
	// payload is the request body exactly as it arrived - the signature covers the bytes, so a
	// body that has been decoded and re-encoded will not verify. It returns ErrInvalidSignature
	// for a delivery this deployment cannot prove came from the provider, and ErrEventIgnored for
	// a verified delivery that says nothing about a subscription of ours, which is most of them:
	// a webhook endpoint is told about far more than it asked for.
	DecodeEvent(payload []byte, signature string) (SubscriptionEvent, error)
}

// CheckoutRequest is one buyer being sent to pay.
type CheckoutRequest struct {
	// UserID is the account the purchase is for, taken from the caller's verified token. It is
	// what comes back on the events the purchase produces, and so is the whole of how a payment
	// finds the profile it paid for.
	UserID string
	// Email prefills the provider's form, saving the buyer from typing an address the credential
	// they signed in with already asserts. It identifies nothing: access follows UserID above.
	Email string
	// SuccessURL and CancelURL are where the provider returns the buyer afterward. Neither grants
	// anything - a buyer who arrives at SuccessURL has been redirected by a browser, not verified,
	// and the subscription is written by the webhook rather than by their return.
	SuccessURL string
	CancelURL  string
}

// SubscriptionEvent is what one verified provider event says about one account's paid access: the
// account, and the access itself.
type SubscriptionEvent struct {
	// UserID is the account the event is about, carried on the subscription since the checkout
	// that created it (see CheckoutRequest.UserID). An event with no account attached is not one
	// this service can act on, and is reported as ErrEventIgnored rather than guessed at.
	UserID string
	entity.Subscription
}
