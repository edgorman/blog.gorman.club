package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// BlogRepository persists blogs. A post is identified by its slug alone - slugs are unique across
// every author, not merely within one - so a lookup names nothing else. Updates are unconditional
// (last writer wins) and Get is unfiltered: callers check read access themselves via
// entity.Blog.CanBeReadBy.
type BlogRepository interface {
	// Get returns ErrNotFound if no undeleted post holds slug.
	Get(ctx context.Context, slug string) (entity.Blog, error)
	// List returns the undeleted blogs uid may read, newest first, applying the same predicate as
	// entity.Blog.CanBeReadBy.
	List(ctx context.Context, uid string) ([]entity.Blog, error)
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
