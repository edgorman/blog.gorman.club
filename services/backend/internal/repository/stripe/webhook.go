package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	// signatureTolerance is how old a delivery may be and still be accepted. The signature covers
	// a timestamp precisely so that a captured delivery cannot be replayed forever - without this
	// window, one valid "subscription renewed" body would be a permanent free subscription for
	// whoever kept a copy of it. Five minutes is Stripe's own recommendation, and is generous
	// enough for a redelivery after a blip.
	signatureTolerance = 5 * time.Minute
	// signatureScheme is the version of Stripe's signing scheme this understands. A header may
	// carry several schemes; anything that is not this one is ignored rather than trusted.
	signatureScheme = "v1"
)

// The event types this service acts on. They are the subscription's own lifecycle rather than the
// checkout's, which is deliberate: checkout.session.completed says a purchase happened but not
// what it bought or for how long, and it never fires again - a renewal twelve months later is a
// subscription event and nothing else. Listening to the subscription means one code path covers
// buying, renewing, lapsing and cancelling, and the first of those needs no special case.
const (
	eventSubscriptionCreated = "customer.subscription.created"
	eventSubscriptionUpdated = "customer.subscription.updated"
	eventSubscriptionDeleted = "customer.subscription.deleted"
)

// The subscription statuses that grant access. Everything else - past_due, unpaid, incomplete,
// canceled, paused - does not, which is the conservative half of each pair: an account whose
// payment is being retried keeps nothing, and gets it back the moment the retry succeeds and
// Stripe says active again. The alternative, serving a paid feature on the strength of a payment
// that has not cleared, is the one that costs money.
var activeStatuses = map[string]bool{"active": true, "trialing": true}

// event is the envelope every Stripe webhook delivery arrives in.
type event struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object subscription `json:"object"`
	} `json:"data"`
}

// subscription is the part of Stripe's subscription object this service reads. Everything else on
// it - the price, the quantity, the discounts, the schedule - is Stripe's business.
type subscription struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// Customer is a bare id on a webhook delivery, since Stripe expands nothing unless asked.
	Customer string            `json:"customer"`
	Metadata map[string]string `json:"metadata"`
	// CurrentPeriodEnd is where the paid-until date lives in older API versions. Stripe moved it
	// onto the individual items in 2025, and a webhook endpoint's version is a setting on the
	// Stripe account rather than something this code can pin from here - so both shapes are read
	// and whichever is present wins. Reading only one would mean an endpoint created against the
	// other version silently granting every subscriber nothing.
	CurrentPeriodEnd int64 `json:"current_period_end"`
	Items            struct {
		Data []struct {
			CurrentPeriodEnd int64 `json:"current_period_end"`
		} `json:"data"`
	} `json:"items"`
}

// periodEnd is when the paid-for period runs out, read from whichever shape the account's API
// version emits. The latest of the items is taken rather than the first: a subscription with more
// than one item is not something this service sells, but if one ever arrives, granting access
// until the last of them expires is the answer that does not cut off somebody who has paid.
func (s subscription) periodEnd() time.Time {
	var latest int64
	for _, item := range s.Items.Data {
		if item.CurrentPeriodEnd > latest {
			latest = item.CurrentPeriodEnd
		}
	}
	if latest == 0 {
		latest = s.CurrentPeriodEnd
	}
	if latest <= 0 {
		return time.Time{}
	}
	return time.Unix(latest, 0).UTC()
}

// DecodeEvent verifies a delivery and translates it into what it says about one account's access.
//
// Verification comes first and unconditionally: the body is not parsed, let alone acted on, until
// the signature proves Stripe sent it. This endpoint is a public URL whose whole job is granting
// paid access, so an unsigned body is not input to be validated - it is a stranger's assertion
// that somebody paid.
func (p *Payments) DecodeEvent(payload []byte, signature string) (repository.SubscriptionEvent, error) {
	if p.cfg.WebhookSecret == "" {
		return repository.SubscriptionEvent{}, repository.ErrPaymentsNotConfigured
	}
	if err := p.verifySignature(payload, signature); err != nil {
		return repository.SubscriptionEvent{}, err
	}

	var delivered event
	if err := json.Unmarshal(payload, &delivered); err != nil {
		return repository.SubscriptionEvent{}, fmt.Errorf("decode event: %w", err)
	}

	switch delivered.Type {
	case eventSubscriptionCreated, eventSubscriptionUpdated, eventSubscriptionDeleted:
	default:
		// Not a failure. A webhook endpoint is told about everything the account is configured to
		// send it, and most of it - invoices, payment intents, customer updates - says nothing
		// this service acts on.
		return repository.SubscriptionEvent{}, fmt.Errorf("event %s is a %s: %w",
			delivered.ID, delivered.Type, repository.ErrEventIgnored)
	}

	object := delivered.Data.Object
	uid := object.Metadata[metadataUserID]
	if uid == "" {
		// A subscription created outside this service's checkout - by hand in the Stripe dashboard,
		// say - carries no account, and there is nothing to guess from: the customer's email is not
		// what access follows here, precisely so that access cannot be claimed by holding an
		// address. Ignored rather than failed, so Stripe stops redelivering something that will
		// never be actionable.
		return repository.SubscriptionEvent{}, fmt.Errorf("subscription %s carries no %s metadata: %w",
			object.ID, metadataUserID, repository.ErrEventIgnored)
	}

	// A deleted subscription is over whatever its period said, and a status outside activeStatuses
	// grants nothing - both leave Until zero, which is the one representation of "not paying"
	// (see entity.Subscription).
	granted := entity.Subscription{CustomerID: object.Customer}
	if delivered.Type != eventSubscriptionDeleted && activeStatuses[object.Status] {
		granted.Until = object.periodEnd()
	}
	return repository.SubscriptionEvent{UserID: uid, Subscription: granted}, nil
}

// verifySignature checks the Stripe-Signature header against the raw body.
//
// The header looks like `t=1614556800,v1=5257a869...`, and what is signed is the timestamp, a
// literal ".", and the body bytes. Two things are load-bearing beyond the HMAC itself: the
// comparison is constant-time, so a wrong signature leaks nothing about how wrong it was; and the
// timestamp is checked against the tolerance above, without which a captured delivery would be
// replayable forever.
func (p *Payments) verifySignature(payload []byte, header string) error {
	timestamp, signatures := parseSignatureHeader(header)
	if timestamp == "" || len(signatures) == 0 {
		return fmt.Errorf("signature header is malformed: %w", repository.ErrInvalidSignature)
	}

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("signature timestamp %q: %w", timestamp, repository.ErrInvalidSignature)
	}
	// Absolute, so a delivery stamped in the future is refused as well as one stamped in the past:
	// a clock skewed the wrong way would otherwise widen the replay window rather than narrow it.
	if age := p.now().Sub(time.Unix(seconds, 0)); age > signatureTolerance || age < -signatureTolerance {
		return fmt.Errorf("signature is %s old: %w", age.Round(time.Second), repository.ErrInvalidSignature)
	}

	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := mac.Sum(nil)

	// Several signatures can be present at once, which is how Stripe rolls a signing secret: both
	// the old and the new one sign a delivery for the length of the rollover. Any one matching is
	// the delivery being genuine.
	for _, candidate := range signatures {
		decoded, err := hex.DecodeString(candidate)
		if err != nil {
			continue
		}
		if hmac.Equal(decoded, expected) {
			return nil
		}
	}
	return fmt.Errorf("no signature matched: %w", repository.ErrInvalidSignature)
}

// parseSignatureHeader splits `t=...,v1=...,v1=...` into its timestamp and its v1 signatures,
// ignoring schemes it does not know.
func parseSignatureHeader(header string) (timestamp string, signatures []string) {
	for _, part := range strings.Split(header, ",") {
		scheme, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch scheme {
		case "t":
			timestamp = value
		case signatureScheme:
			signatures = append(signatures, value)
		}
	}
	return timestamp, signatures
}
