package main

import "context"

// UserStore persists /users/{userId} documents.
type UserStore interface {
	// Get returns ErrNotFound if id doesn't exist.
	Get(ctx context.Context, id string) (User, error)
	// Set creates or overwrites the document at id.
	Set(ctx context.Context, user User) error
}

// BlogStore persists /blogs/{blogId} documents.
type BlogStore interface {
	// Get returns ErrNotFound if id doesn't exist.
	Get(ctx context.Context, id string) (Blog, error)
	// List returns every blog visible to callerUID (see Blog.visibleTo), newest first.
	List(ctx context.Context, callerUID string) ([]Blog, error)
	// Create assigns a new ID and creation/update timestamps.
	Create(ctx context.Context, blog Blog) (Blog, error)
	// Update overwrites the document at blog.ID and refreshes UpdatedAt. Callers are expected to
	// have loaded the blog first (handlers do, to check ownership), so CreatedAt is carried over
	// from blog rather than re-read. Writes unconditionally: last writer wins.
	Update(ctx context.Context, blog Blog) (Blog, error)
	Delete(ctx context.Context, id string) error
}
