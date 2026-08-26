package repository

import "errors"

// ErrNotFound is returned by every Get for an id that doesn't exist, so callers can tell an absent
// record from a failed lookup without knowing which backing store they're talking to.
var ErrNotFound = errors.New("not found")

// ErrAuthNotConfigured means the deployment cannot verify credentials at all - an operator
// problem rather than a caller one.
var ErrAuthNotConfigured = errors.New("authentication is not configured")
