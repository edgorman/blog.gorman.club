package repository

import (
	"context"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// ListParams bounds one page of BlogRepository.List.
type ListParams struct {
	// Limit caps how many posts a page holds. A repository is free to treat a non-positive or
	// out-of-range value as its own default rather than erroring, since this is a page size, not a
	// caller-facing validation.
	Limit int
	// StartAfter, when non-zero, excludes every post at or after this createdAt - continuing a feed
	// a caller has already seen a page of, rather than restarting it. A caller sets it to the
	// createdAt of the last post its previous page held.
	StartAfter time.Time
	// OwnerUID, when set, narrows the page to one author's posts - what a profile feed asks for
	// instead of the general feed's "everything uid may read". It is an id, not a username: a
	// caller that only holds a username resolves it first, the same way any other owner-facing
	// lookup does.
	OwnerUID string
}

// BlogRepository persists blogs. A post is identified by its slug alone - slugs are unique across
// every author, not merely within one - so a lookup names nothing else. Updates are unconditional
// (last writer wins) and Get is unfiltered: callers check read access themselves via
// entity.Blog.CanBeReadBy.
type BlogRepository interface {
	// Get returns ErrNotFound if no undeleted post holds slug.
	Get(ctx context.Context, slug string) (entity.Blog, error)
	// List returns one page of the undeleted blogs uid may read, newest first, applying the same
	// predicate as entity.Blog.CanBeReadBy, and reports whether a further page follows.
	List(ctx context.Context, uid string, params ListParams) (blogs []entity.Blog, hasMore bool, err error)
	// Create writes a new blog at the slug its caller chose, stamping the creation/update
	// timestamps. It rejects a blog that fails entity.Blog.Validate without writing anything, and
	// returns ErrSlugTaken - again without writing - if any post already holds that slug, so the
	// caller can try another (see entity.NewBlogSlug).
	Create(ctx context.Context, blog entity.Blog) (entity.Blog, error)
	// Update overwrites the post at blog.Slug and refreshes UpdatedAt, carrying CreatedAt over from
	// blog rather than re-reading it. It rejects a blog that fails entity.Blog.Validate without
	// writing anything.
	Update(ctx context.Context, blog entity.Blog) (entity.Blog, error)
	// Delete soft-deletes the post at slug by stamping entity.Blog.DeletedAt - the document itself
	// is never removed from Firestore. It returns ErrNotFound if no undeleted post holds slug.
	Delete(ctx context.Context, slug string) error
}
