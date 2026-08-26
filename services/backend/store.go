package main

import "context"

// BlogStore persists /blogs/{blogId} documents. Every write is unconditional (last writer wins)
// and every read is unfiltered - callers check read access themselves via canRead.
type BlogStore interface {
	// Get returns ErrNotFound if id doesn't exist.
	Get(ctx context.Context, id string) (Blog, error)
	// List returns the blogs uid may read, newest first, applying the same predicate as canRead.
	List(ctx context.Context, uid string) ([]Blog, error)
	// Create assigns a new ID and creation/update timestamps.
	Create(ctx context.Context, blog Blog) (Blog, error)
	// Update overwrites the document at blog.ID and refreshes UpdatedAt, carrying CreatedAt over
	// from blog rather than re-reading it.
	Update(ctx context.Context, blog Blog) (Blog, error)
	Delete(ctx context.Context, id string) error
}

// UserStore persists /users/{userId} documents.
type UserStore interface {
	// Get returns ErrNotFound if id doesn't exist.
	Get(ctx context.Context, id string) (User, error)
	// Put writes the document at user.ID, creating it if absent, refreshing UpdatedAt and
	// stamping CreatedAt when it is zero.
	Put(ctx context.Context, user User) (User, error)
	Delete(ctx context.Context, id string) error
}
