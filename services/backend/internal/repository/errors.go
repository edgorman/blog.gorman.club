package repository

import "errors"

// ErrNotFound is returned by every Get for an id that doesn't exist, so callers can tell an absent
// record from a failed lookup without knowing which backing store they're talking to.
var ErrNotFound = errors.New("not found")

// ErrUsernameTaken is returned by UserRepository.Put when the requested username is already held
// by a different user. Uniqueness can only be decided by the write itself, so this is how a caller
// learns to pick another name.
var ErrUsernameTaken = errors.New("username taken")

// ErrSlugTaken is returned by BlogRepository.Create when the author already holds the requested
// slug. Slugs come from post titles, so one author posting twice under a title collides by design;
// as with ErrUsernameTaken, only the write can decide whether a slug is free, so this is how a
// caller learns to draw another.
var ErrSlugTaken = errors.New("slug taken")

// ErrAuthNotConfigured means the deployment cannot verify credentials at all - an operator
// problem rather than a caller one.
var ErrAuthNotConfigured = errors.New("authentication is not configured")

// ErrAssistantNotConfigured means the deployment has no model to call - an operator problem rather
// than a caller one, in the same way ErrAuthNotConfigured is.
var ErrAssistantNotConfigured = errors.New("assistant is not configured")

// ErrPaymentsNotConfigured means the deployment cannot take a payment - no provider credentials,
// or nothing to sell. An operator problem rather than a caller one, like the two above.
var ErrPaymentsNotConfigured = errors.New("payments are not configured")

// ErrInvalidSignature is returned by Payments.DecodeEvent for a delivery this deployment cannot
// prove came from the payment provider. A webhook endpoint is a public URL that grants paid
// access, so a delivery that fails this check is not a malformed request to be tolerated: it is
// either a bug or somebody trying to grant themselves a subscription.
var ErrInvalidSignature = errors.New("invalid signature")

// ErrEventIgnored is returned by Payments.DecodeEvent for a verified delivery that says nothing
// about a subscription this service granted. It is not a failure - a webhook endpoint is told
// about far more than it asked for - so a caller answers it by doing nothing, successfully.
var ErrEventIgnored = errors.New("event ignored")
