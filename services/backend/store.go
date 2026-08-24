package main

import "context"

// BlogStore persists /blogs/{blogId} documents.
type BlogStore interface {
	// Get returns ErrNotFound if id doesn't exist. Callers are responsible for checking the
	// caller may read the result (see canRead) - Get itself is unfiltered.
	Get(ctx context.Context, id string) (Blog, error)
	// List returns the blogs uid is allowed to read, newest first, or an empty slice if there
	// are none. Implementations apply the same predicate as canRead.
	List(ctx context.Context, uid string) ([]Blog, error)
	// Create assigns a new ID and creation/update timestamps.
	Create(ctx context.Context, blog Blog) (Blog, error)
	// Update overwrites the document at blog.ID and refreshes UpdatedAt. Callers are expected to
	// have loaded the blog first (handlers do, to check ownership), so CreatedAt is carried over
	// from blog rather than re-read. Writes unconditionally: last writer wins.
	Update(ctx context.Context, blog Blog) (Blog, error)
	Delete(ctx context.Context, id string) error
}
