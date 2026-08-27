package entity

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxDisplayNameLength = 100
	MaxBioLength         = 500
)

// User is a profile keyed by the owner's Google account ID (the token's `sub` claim), so profiles
// are written with PUT rather than POSTed. Any caller, signed in or not, may read one; only its
// owner may write it.
type User struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	Bio         string    `json:"bio,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
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
	if err := candidate.SetDisplayName(u.DisplayName); err != nil {
		return err
	}
	return candidate.SetBio(u.Bio)
}
