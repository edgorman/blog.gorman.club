package entity

import (
	"fmt"
	"regexp"
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

// blogSlugPattern is what a slug has to look like to be addressable: lowercase alphanumeric words
// joined by single hyphens, with no leading, trailing, or doubled separator. That is exactly the
// shape NewBlogSlug produces, and it also excludes every value Firestore refuses inside a document
// key ("." and "..", anything holding a slash, and the "__reserved__" form).
var blogSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

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
//
// A post has no identifier of its own beyond its author and its slug: the two together address it
// (as /blogs/{username}/{slug}, resolved through the author's profile) and the two together are
// what the repository keys it by, so an opaque id would be a third name for something already
// named twice.
//
// Slug is derived from the title at creation (see NewBlogSlug), so a post is addressed at a URL
// that reads as what it is called. It is assigned once and never revised, so retitling a post
// leaves every link to it working - the title readers see is the stored Title, which is free to
// change beneath a fixed slug.
type Blog struct {
	Slug           string     `json:"slug"`
	OwnerID        string     `json:"ownerId"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	Visibility     Visibility `json:"visibility"`
	AllowedUserIDs []string   `json:"allowedUserIds,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// SetSlug trims and validates a new slug before applying it. Unlike the other setters this one is
// not driven by anything a client sends: slugs are assigned by the server at creation and carried
// over on every write after it, so this guards what the service generates - and what arrives in a
// URL - rather than what a request body holds.
func (b *Blog) SetSlug(slug string) error {
	trimmed := strings.TrimSpace(slug)
	switch {
	case trimmed == "":
		return ValidationError{Field: "slug", Message: "is required"}
	case len(trimmed) > MaxBlogSlugLength:
		return ValidationError{Field: "slug", Message: lengthMessage(MaxBlogSlugLength)}
	case !blogSlugPattern.MatchString(trimmed):
		return ValidationError{Field: "slug", Message: "must be lowercase words of letters and digits joined by single hyphens"}
	}

	b.Slug = trimmed
	return nil
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
	// Bounded before the loop, not after it: de-duplication is quadratic, so cleaning first would
	// let a caller spend arbitrary CPU on a request that is rejected anyway.
	if len(uids) > MaxAllowedUsers {
		return ValidationError{
			Field:   "allowedUserIds",
			Message: fmt.Sprintf("must hold %d entries or fewer", MaxAllowedUsers),
		}
	}

	cleaned := make([]string, 0, len(uids))
	for _, uid := range uids {
		trimmed := strings.TrimSpace(uid)
		if trimmed == "" || slices.Contains(cleaned, trimmed) {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}

	if len(cleaned) == 0 {
		b.AllowedUserIDs = nil
		return nil
	}
	b.AllowedUserIDs = cleaned
	return nil
}

// Validate reports whether the blog is in a storable state: the server-set slug and owner are
// present, and every other field holds a value the setters above would accept. Repositories call
// it before each write, so a blog assembled outside the HTTP layer cannot sidestep the rules -
// including one carrying a slug that could not be addressed, or that Firestore would refuse
// inside a document key.
func (b Blog) Validate() error {
	if b.OwnerID == "" {
		return ValidationError{Field: "ownerId", Message: "is required"}
	}

	candidate := b
	if err := candidate.SetSlug(b.Slug); err != nil {
		return err
	}
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
