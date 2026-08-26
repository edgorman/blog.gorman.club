package entity

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxTitleLength   = 200
	MaxContentLength = 100_000
	MaxAllowedUsers  = 100
)

// Visibility decides who may read a blog beyond its owner.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

func (v Visibility) Valid() bool {
	return v == VisibilityPublic || v == VisibilityPrivate
}

// Blog is a post. This service holds the only credentials for the collection, so read and write
// access is decided here (CanBeReadBy, IsOwnedBy) and nowhere else.
type Blog struct {
	ID             string     `json:"id"`
	OwnerID        string     `json:"ownerId"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	Visibility     Visibility `json:"visibility"`
	AllowedUserIDs []string   `json:"allowedUserIds,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// SetTitle trims and validates a new title before applying it. An empty title is allowed - the
// frontend renders those as "(untitled)".
func (b *Blog) SetTitle(title string) error {
	trimmed := strings.TrimSpace(title)
	if utf8.RuneCountInString(trimmed) > MaxTitleLength {
		return ValidationError{Field: "title", Message: lengthMessage(MaxTitleLength)}
	}

	b.Title = trimmed
	return nil
}

// SetContent validates new content before applying it. Whitespace is significant here, so unlike
// the other setters this one does not trim.
func (b *Blog) SetContent(content string) error {
	if utf8.RuneCountInString(content) > MaxContentLength {
		return ValidationError{Field: "content", Message: lengthMessage(MaxContentLength)}
	}

	b.Content = content
	return nil
}

// SetVisibility validates a new visibility before applying it.
func (b *Blog) SetVisibility(visibility Visibility) error {
	if !visibility.Valid() {
		return ValidationError{
			Field:   "visibility",
			Message: fmt.Sprintf("must be %q or %q", VisibilityPublic, VisibilityPrivate),
		}
	}

	b.Visibility = visibility
	return nil
}

// SetAllowedUserIDs validates and applies the whitelist of uids that may read a private blog,
// dropping blanks and duplicates.
func (b *Blog) SetAllowedUserIDs(uids []string) error {
	cleaned := make([]string, 0, len(uids))
	for _, uid := range uids {
		trimmed := strings.TrimSpace(uid)
		if trimmed == "" || slices.Contains(cleaned, trimmed) {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) > MaxAllowedUsers {
		return ValidationError{
			Field:   "allowedUserIds",
			Message: fmt.Sprintf("must hold %d entries or fewer", MaxAllowedUsers),
		}
	}

	if len(cleaned) == 0 {
		b.AllowedUserIDs = nil
		return nil
	}
	b.AllowedUserIDs = cleaned
	return nil
}

// Validate reports whether every field holds a value the setters above would accept, so the rules
// are defined once and checked the same way whichever entry point a value arrived through.
func (b Blog) Validate() error {
	candidate := b
	if err := candidate.SetTitle(b.Title); err != nil {
		return err
	}
	if err := candidate.SetContent(b.Content); err != nil {
		return err
	}
	if err := candidate.SetVisibility(b.Visibility); err != nil {
		return err
	}
	return candidate.SetAllowedUserIDs(b.AllowedUserIDs)
}

// IsOwnedBy reports whether uid may write this blog.
func (b Blog) IsOwnedBy(uid string) bool {
	return b.OwnerID != "" && b.OwnerID == uid
}

// CanBeReadBy is the single definition of read access: public posts are readable by any signed-in
// caller, private ones only by their owner or a whitelisted uid.
func (b Blog) CanBeReadBy(uid string) bool {
	if b.Visibility == VisibilityPublic || b.IsOwnedBy(uid) {
		return true
	}
	return slices.Contains(b.AllowedUserIDs, uid)
}

func lengthMessage(max int) string {
	return fmt.Sprintf("must be %d characters or fewer", max)
}
