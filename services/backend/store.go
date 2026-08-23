package main

import "context"

// BlogStore persists /blogs/{blogId} documents. Write-only apart from Get, which exists to
// authorize updates and deletes - the frontend reads blogs directly via the Firebase SDK.
type BlogStore interface {
	// Get returns ErrNotFound if id doesn't exist.
	Get(ctx context.Context, id string) (Blog, error)
	// Create assigns a new ID and creation/update timestamps.
	Create(ctx context.Context, blog Blog) (Blog, error)
	// Update overwrites the document at blog.ID and refreshes UpdatedAt. Callers are expected to
	// have loaded the blog first (handlers do, to check ownership), so CreatedAt is carried over
	// from blog rather than re-read. Writes unconditionally: last writer wins.
	Update(ctx context.Context, blog Blog) (Blog, error)
	Delete(ctx context.Context, id string) error
}
