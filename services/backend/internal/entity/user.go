package entity

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MinUsernameLength    = 3
	MaxUsernameLength    = 30
	MaxDisplayNameLength = 100
	MaxBioLength         = 500
)

// usernamePattern is deliberately ASCII-only: usernames end up in URLs and are how one person
// refers to another, so the alphabet stays narrow enough that two names cannot look alike.
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// User is a profile keyed by the owner's Google account ID (the token's `sub` claim), so profiles
// are written with PUT rather than POSTed. Any caller, signed in or not, may read one; only its
// owner may write it. Username is the handle a profile is looked up by instead of that opaque id,
// and is assigned at sign-up by NewUsername.
type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Bio         string    `json:"bio,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SetUsername trims and validates a new username before applying it.
func (u *User) SetUsername(username string) error {
	trimmed := strings.TrimSpace(username)
	// The pattern is checked before the lengths so they are measured on a value already known to be
	// ASCII, where a byte and a character are the same thing.
	switch {
	case trimmed == "":
		return ValidationError{Field: "username", Message: "is required"}
	case !usernamePattern.MatchString(trimmed):
		return ValidationError{Field: "username", Message: "must contain only letters, digits, and hyphens"}
	case len(trimmed) < MinUsernameLength:
		return ValidationError{Field: "username", Message: minLengthMessage(MinUsernameLength)}
	case len(trimmed) > MaxUsernameLength:
		return ValidationError{Field: "username", Message: lengthMessage(MaxUsernameLength)}
	}

	u.Username = trimmed
	return nil
}

// UsernameKey is the form uniqueness is enforced on. Folding case here is what stops "Ed" and "ed"
// from being claimed as two different handles, while the username itself is stored as it was typed.
func (u User) UsernameKey() string {
	return strings.ToLower(u.Username)
}

// SetDisplayName trims and validates a new display name before applying it.
func (u *User) SetDisplayName(name string) error {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return ValidationError{Field: "displayName", Message: "is required"}
	case utf8.RuneCountInString(trimmed) > MaxDisplayNameLength:
		return ValidationError{Field: "displayName", Message: lengthMessage(MaxDisplayNameLength)}
	}

	u.DisplayName = trimmed
	return nil
}

// SetBio trims and validates a new bio before applying it. An empty bio is allowed.
func (u *User) SetBio(bio string) error {
	trimmed := strings.TrimSpace(bio)
	if utf8.RuneCountInString(trimmed) > MaxBioLength {
		return ValidationError{Field: "bio", Message: lengthMessage(MaxBioLength)}
	}

	u.Bio = trimmed
	return nil
}

// Validate reports whether the profile is in a storable state: the id is present, and every other
// field holds a value the setters above would accept. Repositories call it before each write, so a
// profile assembled outside the HTTP layer cannot sidestep the rules.
func (u User) Validate() error {
	if u.ID == "" {
		return ValidationError{Field: "id", Message: "is required"}
	}

	candidate := u
	if err := candidate.SetUsername(u.Username); err != nil {
		return err
	}
	if err := candidate.SetDisplayName(u.DisplayName); err != nil {
		return err
	}
	return candidate.SetBio(u.Bio)
}
