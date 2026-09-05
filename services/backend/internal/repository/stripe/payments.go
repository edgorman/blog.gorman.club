// Package stripe takes subscription payments through Stripe: a hosted Checkout page to buy one,
// and a signed webhook to be told what was bought.
//
// It talks to Stripe's REST API over net/http rather than through Stripe's Go SDK, for the same
// reason the gemini package speaks the platform's REST protocol directly: what this service asks
// of a payment provider is two calls wide (see repository.Payments), the API surface behind them
// is form-encoded parameters and JSON back, and the SDK's value is in the hundred endpoints and
// dozens of typed objects that are not used here. A dependency that ships the whole of Stripe's
// object model to spare this package one url.Values is not a trade worth making, and it would be
// the largest thing in go.mod by a wide margin.
//
// Two things follow from that choice and are worth stating, because they are what the SDK would
// otherwise have handled:
//
//   - No API version is pinned on outbound calls. The parameters below (mode, line_items,
//     success_url) have been stable across every version that has a hosted Checkout, so there is
//     nothing to pin them against; and pinning would not help with events anyway, since a webhook
//     endpoint's version is a setting on the Stripe account rather than something a request can
//     ask for. What the event decoder does instead is read both shapes of the one field that has
//     moved between versions (see webhook.go).
//   - Signature verification is implemented here rather than imported. It is thirty lines of HMAC
//     and a timestamp check, and it is the whole of what stands between a public URL and free
//     subscriptions, so it is worth having in the open where it can be read and tested.
package stripe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	// defaultBaseURL is Stripe's API host. Config.BaseURL overrides it for tests.
	defaultBaseURL = "https://api.stripe.com"
	// requestTimeout bounds one call to Stripe. Creating a checkout session happens while a buyer
	// waits on a button, so this is short: a Stripe that has not answered by now is one this
	// request should give up on rather than one worth holding the browser open for.
	requestTimeout = 20 * time.Second
	// maxErrorBody bounds how much of a failed response is read before giving up on parsing it,
	// so a Stripe outage that answers with an HTML error page cannot be logged in full.
	maxErrorBody = 4 << 10
	// metadataUserID is the key an account's uid travels under, set on the subscription when the
	// checkout is created and read back off every subscription event. It is the whole of the join
	// between a payment and a profile: Stripe knows an email and a customer id, neither of which
	// this service keys access on.
	metadataUserID = "userId"
)

var _ repository.Payments = (*Payments)(nil)

// Config is what this deployment needs to take a payment.
type Config struct {
	// SecretKey is the Stripe API key (sk_test_... in staging, sk_live_... in production). It is
	// the one long-lived credential in this backend, which is why it is read from an environment
	// variable sourced from Secret Manager rather than baked into the image, and why it is
	// per-environment: staging holds a test-mode key that cannot move real money.
	//
	// Unlike the model credentials in the gemini package there is no federated alternative to
	// reach for - Stripe has no equivalent of Workload Identity Federation - so the secret is
	// handled the way the repository handles the one other unavoidable secret it has: stored in
	// GCP Secret Manager, mounted by Cloud Run, never in Terraform state or a GitHub secret.
	SecretKey string
	// WebhookSecret is the signing secret of the webhook endpoint Stripe delivers to (whsec_...).
	// It is not the API key and is not interchangeable with it: this one only ever verifies, and a
	// deployment with no webhook secret can sell a subscription but cannot be told about one -
	// which is why Configured requires both.
	WebhookSecret string
	// PriceID is what is being sold (price_...). There is exactly one, because there is exactly
	// one tier: an account has paid or it has not (see entity.Subscription).
	PriceID string
	// BaseURL overrides Stripe's host, for tests pointing at an httptest server.
	BaseURL string
	// HTTPClient overrides the client used for API calls, for the same reason.
	HTTPClient *http.Client
	// Now is the clock signature verification checks a delivery's timestamp against, swappable so
	// a test can hold a fixed signature and still exercise the tolerance window.
	Now func() time.Time
}

// Payments implements repository.Payments against Stripe.
type Payments struct {
	cfg    Config
	client *http.Client
	now    func() time.Time
}

// NewPayments returns a Payments for cfg. It performs no I/O and never fails: a deployment with no
// credentials is reported by Configured, and every method refuses with
// repository.ErrPaymentsNotConfigured, which is how the other adapters here report the same thing.
func NewPayments(cfg Config) *Payments {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Payments{cfg: cfg, client: client, now: now}
}

// Configured reports whether this deployment can both sell a subscription and be told about one.
// All three values are required, because two out of three is worse than none: a checkout that
// nothing is listening for takes a buyer's money and grants them nothing.
func (p *Payments) Configured() bool {
	return p.cfg.SecretKey != "" && p.cfg.WebhookSecret != "" && p.cfg.PriceID != ""
}

func (p *Payments) baseURL() string {
	if p.cfg.BaseURL != "" {
		return strings.TrimSuffix(p.cfg.BaseURL, "/")
	}
	return defaultBaseURL
}

// Checkout creates a Stripe Checkout session and returns the URL to send the buyer to.
//
// The uid is attached twice on purpose. client_reference_id is what a person reading the Stripe
// dashboard sees against the session, and subscription_data[metadata] is what rides on the
// subscription itself and so comes back on every event about it - including renewals years later,
// long after the session is gone. Only the second one is load-bearing (see webhook.go); the first
// is what makes a support question answerable.
func (p *Payments) Checkout(ctx context.Context, req repository.CheckoutRequest) (string, error) {
	if !p.Configured() {
		return "", repository.ErrPaymentsNotConfigured
	}
	if req.UserID == "" {
		// Refused rather than sent without one: a subscription whose events carry no uid grants
		// nobody anything, so this would take a payment for nothing.
		return "", fmt.Errorf("checkout: no user id")
	}

	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("line_items[0][price]", p.cfg.PriceID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("success_url", req.SuccessURL)
	form.Set("cancel_url", req.CancelURL)
	form.Set("client_reference_id", req.UserID)
	form.Set("subscription_data[metadata]["+metadataUserID+"]", req.UserID)
	if req.Email != "" {
		form.Set("customer_email", req.Email)
	}

	session, err := p.postForm(ctx, "/v1/checkout/sessions", form)
	if err != nil {
		return "", fmt.Errorf("checkout: %w", err)
	}
	return session, nil
}

// postForm sends a form-encoded request to Stripe and returns the `url` of the session it created,
// which is the only field either of the two calls above reads: both of them create somewhere to
// send a person, and everything else Stripe answers with is about a session this service does not
// keep.
func (p *Payments) postForm(ctx context.Context, path string, form url.Values) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+p.cfg.SecretKey)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := p.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("stripe returned %d: %s", response.StatusCode, errorMessage(response.Body))
	}

	var session struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		return "", fmt.Errorf("decode session: %w", err)
	}
	if session.URL == "" {
		// A session with nowhere to send the person is a success this caller cannot use, so it is
		// reported as the failure it is rather than as an empty redirect.
		return "", fmt.Errorf("stripe returned a session with no url")
	}
	return session.URL, nil
}

// BillingPortal creates a Stripe billing portal session for an existing customer.
//
// The portal is Stripe's own page, and using it rather than building one here is the same decision
// as using hosted Checkout rather than taking a card number: managing a subscription means showing
// invoices, taking a new card, and cancelling, none of which this service should be in the way of.
// A cancellation made there arrives back as an ordinary subscription event, so it is recorded by
// the same webhook path that recorded the purchase.
//
// It requires a portal configuration to exist on the Stripe account - Stripe refuses the call
// otherwise, and says so in the error this returns.
func (p *Payments) BillingPortal(ctx context.Context, customerID, returnURL string) (string, error) {
	if !p.Configured() {
		return "", repository.ErrPaymentsNotConfigured
	}
	if customerID == "" {
		// An account that has never reached a checkout has no customer to manage, and asking
		// Stripe about an empty one would only be a slower way of finding that out.
		return "", fmt.Errorf("billing portal: no customer id")
	}

	form := url.Values{}
	form.Set("customer", customerID)
	form.Set("return_url", returnURL)

	session, err := p.postForm(ctx, "/v1/billing_portal/sessions", form)
	if err != nil {
		return "", fmt.Errorf("billing portal: %w", err)
	}
	return session, nil
}

// errorMessage pulls Stripe's own description out of a failed response, falling back to the raw
// body so an error page from something that is not Stripe still says something.
func errorMessage(body io.Reader) string {
	raw, err := io.ReadAll(io.LimitReader(body, maxErrorBody))
	if err != nil || len(raw) == 0 {
		return "no response body"
	}

	var failure struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &failure); err == nil && failure.Error.Message != "" {
		return failure.Error.Message
	}
	return strings.TrimSpace(string(raw))
}
