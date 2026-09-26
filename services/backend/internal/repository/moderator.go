package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// Moderator screens a comment's body. Like Assistant it only answers: the worker decides what to
// write. An error means the comment could not be classified - the model failed or answered
// off-schema - and is never an approval.
type Moderator interface {
	Classify(ctx context.Context, body string) (entity.Moderation, error)
}
