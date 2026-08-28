package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// BlogRepository persists blogs. A post is identified by its owner and its slug together - slugs
// are unique per author, not globally - so every lookup names both. Updates are unconditional
// (last writer wins) and Get is unfiltered: callers check read access themselves via
// entity.Blog.CanBeReadBy.
type BlogRepository interface {
	// Get returns ErrNotFound if ownerID holds no undeleted post at slug.
	Get(ctx context.Context, ownerID, slug string) (entity.Blog, error)
	// List returns the undeleted blogs uid may read, newest first, applying the same predicate as
	// entity.Blog.CanBeReadBy.
	List(ctx context.Context, uid string) ([]entity.Blog, error)
	// Create writes a new blog at the slug its caller chose, stamping the creation/update
	// timestamps. It rejects a blog that fails entity.Blog.Validate without writing anything, and
	// returns ErrSlugTaken - again without writing - if the owner already holds that slug, so the
	// caller can try another (see entity.NewBlogSlug).
	Create(ctx context.Context, blog entity.Blog) (entity.Blog, error)
	// Update overwrites the owner's post at blog.Slug and refreshes UpdatedAt, carrying CreatedAt
	// over from blog rather than re-reading it. It rejects a blog that fails entity.Blog.Validate
	// without writing anything.
	Update(ctx context.Context, blog entity.Blog) (entity.Blog, error)
	// Delete soft-deletes the owner's post at slug by stamping entity.Blog.DeletedAt - the document
	// itself is never removed from Firestore. It returns ErrNotFound if ownerID holds no undeleted
	// post at slug.
	Delete(ctx context.Context, ownerID, slug string) error
}
