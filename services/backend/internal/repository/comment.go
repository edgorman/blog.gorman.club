package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// CommentRepository persists the comments on a post. A comment is addressed by the pair
// (blogSlug, id): it has no name of its own, and no id here means anything without the post it
// hangs off.
//
// There is no Update, for the same reason entity.Comment has no setter a client can reach: a
// comment is written and removed, never rewritten. Reads are unfiltered - callers decide access
// through the post, via entity.Blog.CanBeReadBy, and deletion through entity.Comment.Permission.
type CommentRepository interface {
	// List returns the comments on blogSlug, oldest first, so a client renders them in the order
	// they were written. It answers an empty slice for a post nobody has commented on rather than
	// ErrNotFound: that is not a missing thread, it is an empty one.
	List(ctx context.Context, blogSlug string) ([]entity.Comment, error)
	// Get returns ErrNotFound if no comment on blogSlug holds id. It exists because deletion is
	// authorized against the comment itself (its author) as well as the post (its owner), so the
	// comment has to be read before it can be removed.
	Get(ctx context.Context, blogSlug, id string) (entity.Comment, error)
	// Create writes a new comment beneath the post it names, assigning its id and stamping
	// CreatedAt, and returns it with both filled in. It rejects a comment that fails
	// entity.Comment.Validate without writing anything.
	Create(ctx context.Context, comment entity.Comment) (entity.Comment, error)
	// Delete removes the comment. Unlike a post it is erased rather than marked gone: a post is a
	// published thing whose absence would be a hole in the record, while a comment being taken
	// down - by whoever wrote it or by the author moderating their own post - has to actually
	// remove what was said.
	Delete(ctx context.Context, blogSlug, id string) error
}
