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
