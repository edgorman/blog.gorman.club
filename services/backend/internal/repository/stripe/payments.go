// Package stripe sells the subscription through Stripe Checkout and reads it back, speaking Stripe's
// REST API over net/http rather than through the Stripe SDK - for the same reason
// internal/repository/gemini hand-writes its protocol: the three calls this service makes are small
// enough to read in one file, and an SDK would pin a dependency to whatever Stripe ships next.
package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	// apiVersion pins the shape of every response read below, whatever the account's default is.
	// From this version on, a subscription's period end lives on its items rather than on the
	// subscription itself; subscriptionResponse reads both, so an older pin would still parse.
	apiVersion = "2025-03-31.basil"
	// requestTimeout bounds one call to Stripe.
	requestTimeout = 20 * time.Second
	// webhookTolerance is the replay window: a correctly signed delivery older (or newer) than this
	// is refused, so a captured one cannot be replayed later. Five minutes is Stripe's own default.
	webhookTolerance = 5 * time.Minute
	// uidMetadataKey is where checkout records the account on the subscription, which is how the
	// webhook finds the account again from a subscription alone.
	uidMetadataKey = "uid"
)

var _ repository.Payments = (*Payments)(nil)

// Config holds the keys and the price. Any of the three empty leaves the deployment without
// billing (see Configured).
type Config struct {
	SecretKey     string
	WebhookSecret string
	// PriceID is the one recurring Price sold. There is one tier, so this is the whole catalogue.
	PriceID string
	// BaseURL and HTTPClient override the API host and client, for tests.
	BaseURL    string
	HTTPClient *http.Client
}

// Payments implements repository.Payments against Stripe.
type Payments struct {
	cfg Config
}

func NewPayments(cfg Config) *Payments {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.stripe.com"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: requestTimeout}
	}
	return &Payments{cfg: cfg}
}

func (p *Payments) Configured() bool {
	return p.cfg.SecretKey != "" && p.cfg.WebhookSecret != "" && p.cfg.PriceID != ""
}

// sessionResponse is the part of a Checkout or Customer Portal session this reads.
type sessionResponse struct {
	URL string `json:"url"`
}

func (p *Payments) CheckoutURL(ctx context.Context, req repository.CheckoutRequest) (string, error) {
	form := url.Values{
		"mode":                    {"subscription"},
		"line_items[0][price]":    {p.cfg.PriceID},
		"line_items[0][quantity]": {"1"},
		"client_reference_id":     {req.UID},
		"success_url":             {req.SuccessURL},
		"cancel_url":              {req.CancelURL},
		"subscription_data[metadata][" + uidMetadataKey + "]": {req.UID},
	}
	if req.CustomerID != "" {
		form.Set("customer", req.CustomerID)
	}

	var session sessionResponse
	if err := p.call(ctx, http.MethodPost, "/v1/checkout/sessions", form, &session); err != nil {
		return "", err
	}
	return session.URL, nil
}

func (p *Payments) PortalURL(ctx context.Context, customerID, returnURL string) (string, error) {
	form := url.Values{"customer": {customerID}, "return_url": {returnURL}}

	var session sessionResponse
	if err := p.call(ctx, http.MethodPost, "/v1/billing_portal/sessions", form, &session); err != nil {
		return "", err
	}
	return session.URL, nil
}

// subscriptionResponse is the part of a Subscription object this reads.
type subscriptionResponse struct {
	ID               string            `json:"id"`
	Customer         string            `json:"customer"`
	Status           string            `json:"status"`
	Metadata         map[string]string `json:"metadata"`
	CurrentPeriodEnd int64             `json:"current_period_end"`
	Items            struct {
		Data []struct {
			CurrentPeriodEnd int64 `json:"current_period_end"`
		} `json:"data"`
	} `json:"items"`
}

// periodEnd is the latest period end across the subscription and its items. There is one item in
// a one-tier subscription, so the latest is simply that item's.
func (s subscriptionResponse) periodEnd() time.Time {
	end := s.CurrentPeriodEnd
	for _, item := range s.Items.Data {
		end = max(end, item.CurrentPeriodEnd)
	}
	return time.Unix(end, 0).UTC()
}

func (p *Payments) Subscription(ctx context.Context, id string) (repository.Subscription, error) {
	var sub subscriptionResponse
	if err := p.call(ctx, http.MethodGet, "/v1/subscriptions/"+url.PathEscape(id), nil, &sub); err != nil {
		return repository.Subscription{}, err
	}
	return repository.Subscription{
		ID:               sub.ID,
		CustomerID:       sub.Customer,
		UID:              sub.Metadata[uidMetadataKey],
		Status:           sub.Status,
		CurrentPeriodEnd: sub.periodEnd(),
	}, nil
}

// call makes one form-encoded request and decodes a 2xx response into out.
func (p *Payments) call(ctx context.Context, method, path string, form url.Values, out any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(p.cfg.BaseURL, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.SecretKey)
	req.Header.Set("Stripe-Version", apiVersion)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("stripe %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		// Stripe's error body names the problem without echoing the key, so it is safe to log.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("stripe %s %s: %s: %s", method, path, resp.Status, detail)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// eventPayload is the part of a webhook delivery this reads. Only the object's id is taken from
// it: the service re-fetches the subscription rather than trusting the rest (see StripeWebhook).
type eventPayload struct {
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		} `json:"object"`
	} `json:"data"`
}

// VerifyWebhook checks the Stripe-Signature header - `t=<unix time>,v1=<hex HMAC>[,v1=...]` - over
// the raw payload before anything in it is parsed. It is the whole of what stands between a public
// URL and free subscriptions, so every way it can fail is a refusal:
//
//   - the signature is HMAC-SHA256 with the webhook secret over "<t>.<payload>", compared in
//     constant time against every v1 entry (Stripe sends more than one while a secret is rolled);
//   - t must be within webhookTolerance of now, so a captured delivery cannot be replayed later.
func (p *Payments) VerifyWebhook(payload []byte, signature string, now time.Time) (repository.WebhookEvent, error) {
	if p.cfg.WebhookSecret == "" {
		return repository.WebhookEvent{}, repository.ErrInvalidSignature
	}

	var timestamp string
	var candidates [][]byte
	for _, field := range strings.Split(signature, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(field), "=")
		switch key {
		case "t":
			timestamp = value
		case "v1":
			if decoded, err := hex.DecodeString(value); err == nil {
				candidates = append(candidates, decoded)
			}
		}
	}

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || len(candidates) == 0 {
		return repository.WebhookEvent{}, repository.ErrInvalidSignature
	}
	if age := now.Sub(time.Unix(seconds, 0)); age > webhookTolerance || age < -webhookTolerance {
		return repository.WebhookEvent{}, repository.ErrInvalidSignature
	}

	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(payload)
	expected := mac.Sum(nil)

	matched := false
	for _, candidate := range candidates {
		// Every candidate is compared, rather than stopping at the first match, so the time taken
		// says nothing about which one matched.
		if hmac.Equal(expected, candidate) {
			matched = true
		}
	}
	if !matched {
		return repository.WebhookEvent{}, repository.ErrInvalidSignature
	}

	var event eventPayload
	if err := json.Unmarshal(payload, &event); err != nil {
		return repository.WebhookEvent{}, fmt.Errorf("decode webhook: %w", err)
	}
	verified := repository.WebhookEvent{Type: event.Type}
	if event.Data.Object.Object == "subscription" {
		verified.SubscriptionID = event.Data.Object.ID
	}
	return verified, nil
}
