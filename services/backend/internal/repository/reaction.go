package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// ReactionRepository persists the reactions on a post and on its comments. Both live beneath the
// post, so one read answers what a reader opening it needs to render: every reaction on the page,
// rather than one query for the post and another per comment.
//
// There is no Get for a single reader's reactions: nothing renders one reader's row on its own -
// a bar is a count and whether you are in it - so the whole set is read and folded together by the
// caller.
type ReactionRepository interface {
	// List returns every reaction on blogSlug and on its comments. A post nobody has reacted to is
	// an empty slice rather than ErrNotFound: an empty bar is not a missing one.
	List(ctx context.Context, blogSlug string) ([]entity.Reaction, error)
	// Add records uid reacting to target with emoji, and returns that reader's reactions to the
	// target as they now stand. It is idempotent - an emoji already there is left alone - and
	// rejects one that fails entity.Reaction.Validate without writing anything.
	Add(ctx context.Context, target entity.ReactionTarget, uid, emoji string) (entity.Reaction, error)
	// Remove takes emoji back, and is idempotent in the same way: one that was never there leaves
	// the reader where they wanted to be. A reader with nothing left on the target is erased
	// rather than stored as an empty record.
	Remove(ctx context.Context, target entity.ReactionTarget, uid, emoji string) (entity.Reaction, error)
	// DeleteTarget removes everybody's reactions to one target. It is what a deleted comment takes
	// with it: reactions to something that no longer exists would be counted by nothing and shown
	// nowhere, and leaving them would let a moderated comment survive as a row of numbers.
	DeleteTarget(ctx context.Context, target entity.ReactionTarget) error
}
