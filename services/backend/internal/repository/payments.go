package repository

import (
	"context"
	"errors"
	"time"
)

// ErrInvalidSignature is returned by Payments.VerifyWebhook for a delivery that did not come from
// the payment provider: a missing or malformed signature, one made with another secret, a body
// changed after signing, or a timestamp outside the replay window.
var ErrInvalidSignature = errors.New("invalid webhook signature")

// Payments sells the one subscription tier. Like Assistant it is deliberately narrow - open a
// hosted page, read a subscription back, check a webhook came from the provider - so everything
// provider-specific (its REST API, its signature scheme) lives behind it in internal/repository/stripe.
type Payments interface {
	// Configured reports whether the deployment has the keys and price it needs. The billing routes
	// are not served at all when it is false.
	Configured() bool
	// CheckoutURL opens a hosted checkout for the subscription and returns where to send the
	// browser.
	CheckoutURL(ctx context.Context, req CheckoutRequest) (string, error)
	// PortalURL opens the hosted page where customerID manages (and cancels) its subscription.
	PortalURL(ctx context.Context, customerID, returnURL string) (string, error)
	// Subscription fetches a subscription's current state from the provider, which is what the
	// webhook acts on rather than the event payload (see the service's StripeWebhook).
	Subscription(ctx context.Context, id string) (Subscription, error)
	// VerifyWebhook checks a delivery's signature header against its raw body, as received at now,
	// and only then parses it. It returns ErrInvalidSignature for anything that fails the check.
	VerifyWebhook(payload []byte, signature string, now time.Time) (WebhookEvent, error)
}

// CheckoutRequest is what a checkout needs to know about the account buying.
type CheckoutRequest struct {
	// UID is the account the subscription is for. It is recorded on the subscription so the
	// webhook can find the account again.
	UID string
	// CustomerID reuses an existing customer, empty for an account that has never paid.
	CustomerID string
	// SuccessURL and CancelURL are where the provider sends the browser back to.
	SuccessURL string
	CancelURL  string
}

// Subscription is a subscription as the provider currently has it.
type Subscription struct {
	ID         string
	CustomerID string
	// UID is the account recorded at checkout, empty if the subscription was not made by one.
	UID string
	// Status is the provider's own status string, e.g. "active" or "canceled".
	Status string
	// CurrentPeriodEnd is when the period already paid for runs out.
	CurrentPeriodEnd time.Time
}

// WebhookEvent is the part of a verified delivery the service acts on.
type WebhookEvent struct {
	Type string
	// SubscriptionID is the subscription the event is about, empty for an event about anything
	// else.
	SubscriptionID string
}
