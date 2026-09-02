package entity

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MinUsernameLength = 3
	MaxUsernameLength = 30
	MaxBioLength      = 500
)

// usernamePattern is deliberately ASCII-only: a username is the whole of a public identity here,
// so the alphabet stays narrow enough that two names cannot look alike.
var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// reservedUsernames are names no profile may hold because a route already means something else at
// that path: "me" addresses the caller's own profile, so a user holding it would be unreachable at
// their own URL. The minimum length happens to exclude it as well; naming it here keeps the rule
// from depending on that coincidence.
//
// "edit" used to sit here too, back when the frontend edited a profile at /profile/edit and that
// literal outranked the username wildcard beside it. The editor now lives under the profile it
// edits (/user/{username}/edit), so "edit" only ever appears as a segment after a username and no
// longer competes with one.
var reservedUsernames = map[string]bool{"me": true}

// User is a profile keyed by the owner's Google account ID (the token's `sub` claim), so profiles
// are written with PUT rather than POSTed. Any caller, signed in or not, may read one; only its
// owner may write it.
//
// Username is the whole of a profile's public identity - both the handle it is looked up by and
// the name readers see. There is deliberately no separate display name: one that could be set
// freely would let anyone present themselves under a name somebody else holds, which is exactly
// what making the unique handle the visible one prevents.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Bio       string    `json:"bio,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// SubscribedUntil is when this account's paid access runs out, and is nil for an account that
	// has never subscribed. It is the seam the payment provider writes to: nothing in this service
	// sets it yet, and everything that asks whether an account has paid asks it (see Subscribed and
	// AssistantEntitlement), so wiring a checkout up means writing this field and nothing else.
	//
	// It carries no json tag because a profile is public and who is paying is nobody else's
	// business: it is reported only to the account itself, by the service's /users/me response,
	// and never by a lookup of somebody else's name.
	SubscribedUntil *time.Time `json:"-"`
}

// Subscribed reports whether the account's paid access is live at now. An account that never
// subscribed and one whose subscription has run out are the same answer, deliberately: the only
// thing anything here needs to know is whether it may spend, and "lapsed" is a billing question
// rather than an access one.
func (u User) Subscribed(now time.Time) bool {
	return u.SubscribedUntil != nil && now.Before(*u.SubscribedUntil)
}

// Permission is the single definition of who may do what to a profile: anybody may read one, since
// a username is the whole of a public identity and a post is shown by its author's name; only the
// account itself may write or delete it, which the /users/me address makes structural.
func (u User) Permission(action Action) Permission {
	permission := PermissionFor(ResourceUser, action)
	permission.OwnerID = u.ID
	return permission
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
	case reservedUsernames[strings.ToLower(trimmed)]:
		return ValidationError{Field: "username", Message: "is reserved"}
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

// SetBio trims and validates a new bio before applying it. An empty bio is allowed.
func (u *User) SetBio(bio string) error {
	trimmed := strings.TrimSpace(bio)
	if utf8.RuneCountInString(trimmed) > MaxBioLength {
		return ValidationError{Field: "bio", Message: lengthMessage(MaxBioLength)}
	}

	u.Bio = trimmed
	return nil
}

// Normalized returns the profile as it should be stored: the id present, and every other field put
// back through its setter, so what is written is what those setters produce rather than whatever
// was handed in. It fails for the same reasons they do.
//
// Returning the normalized copy rather than only checking it is what stops a profile assembled
// outside the HTTP layer from being stored raw - " alice " would otherwise pass validation and be
// written untrimmed, under a reservation key no lookup could match.
func (u User) Normalized() (User, error) {
	if u.ID == "" {
		return User{}, ValidationError{Field: "id", Message: "is required"}
	}

	candidate := u
	if err := candidate.SetUsername(u.Username); err != nil {
		return User{}, err
	}
	if err := candidate.SetBio(u.Bio); err != nil {
		return User{}, err
	}
	return candidate, nil
}

// Validate reports whether the profile is in a storable state, discarding the normalized form.
func (u User) Validate() error {
	_, err := u.Normalized()
	return err
}
