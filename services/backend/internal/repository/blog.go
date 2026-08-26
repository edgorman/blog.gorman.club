package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// BlogRepository persists blogs. Every write is unconditional (last writer wins) and Get is
// unfiltered - callers check read access themselves via entity.Blog.CanBeReadBy.
type BlogRepository interface {
	// Get returns ErrNotFound if id doesn't exist.
	Get(ctx context.Context, id string) (entity.Blog, error)
	// List returns the blogs uid may read, newest first, applying the same predicate as
	// entity.Blog.CanBeReadBy.
	List(ctx context.Context, uid string) ([]entity.Blog, error)
	// Create assigns a new ID and creation/update timestamps.
	Create(ctx context.Context, blog entity.Blog) (entity.Blog, error)
	// Update overwrites the record at blog.ID and refreshes UpdatedAt, carrying CreatedAt over
	// from blog rather than re-reading it.
	Update(ctx context.Context, blog entity.Blog) (entity.Blog, error)
	Delete(ctx context.Context, id string) error
}
