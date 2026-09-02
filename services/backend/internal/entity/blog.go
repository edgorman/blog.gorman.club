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
// access is decided here (Permission, and CanBeReadBy over it) and nowhere else.
//
// A post has no identifier of its own beyond its slug: the slug alone addresses it (as
// /blogs/{slug}) and is what the repository keys it by, so an opaque id would be a second name for
// something already named. Slugs are unique across every author rather than per author, which is
// what lets the author be left out of the address entirely - OwnerID says who wrote a post, not
// where it lives.
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
	// DeletedAt is nil for a live post. A post is never removed from Firestore, only marked gone -
	// so this is the sole record of deletion, and its absence is what "not deleted" means.
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
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
	case reservedBlogSlugs[trimmed]:
		return ValidationError{Field: "slug", Message: "is reserved"}
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
// including one carrying a slug that could not be addressed, that a frontend route already
// claims, or that Firestore would refuse inside a document key.
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

// IsDeleted reports whether the post has been soft-deleted.
func (b Blog) IsDeleted() bool {
	return b.DeletedAt != nil
}

// IsOwnedBy reports whether uid wrote this blog. It is a fact about the post rather than a
// permission - what owning one entitles you to is Permission's answer, and this is what that
// answer is largely made of.
func (b Blog) IsOwnedBy(uid string) bool {
	return b.OwnerID != "" && b.OwnerID == uid
}

// readAccess is a post's visibility said in the vocabulary of the access model: a public post's
// audience is everybody, and a private one's is its owner - widened to a whitelist exactly when
// the post names readers.
//
// This is the one place a resource decides its own audience rather than taking the fixed one the
// policy table declares, and it is what visibility has always meant: a post is the only thing here
// whose author chooses who may see it. Anything the setters would refuse reads as private, so a
// document holding a visibility this build does not know is closed rather than open.
func (b Blog) readAccess() (Access, []string) {
	switch {
	case b.Visibility == VisibilityPublic:
		return AccessPublic, nil
	case len(b.AllowedUserIDs) == 0:
		return AccessPrivate, nil
	default:
		return AccessWhitelist, b.AllowedUserIDs
	}
}

// Permission is the single definition of who may do what to a post: reading follows the post's own
// visibility, and writing it - creating, updating, deleting - is the owner's alone.
//
// Creating is answered here too, against a post that does not exist yet: one is created owned by
// whoever asked, so the permission it is checked against is the same private one an update gets,
// on a Blog whose OwnerID is already the caller. What that actually excludes is the caller with no
// uid at all.
func (b Blog) Permission(action Action) Permission {
	permission := PermissionFor(ResourceBlog, action)
	permission.OwnerID = b.OwnerID
	if action == ActionRead {
		permission.Access, permission.AllowedUserIDs = b.readAccess()
	}
	return permission
}

// CanBeReadBy reports whether uid may see this post, which is Permission(ActionRead) asked by its
// most common name. Repositories use it to filter what they hand back, so it stays a method on the
// post rather than something every caller assembles.
func (b Blog) CanBeReadBy(uid string) bool {
	return b.Permission(ActionRead).Allows(uid)
}
