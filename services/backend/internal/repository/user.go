package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// UserRepository persists user profiles.
type UserRepository interface {
	// Get returns ErrNotFound if id doesn't exist.
	Get(ctx context.Context, id string) (entity.User, error)
	// Put writes the record at user.ID, creating it if absent, refreshing UpdatedAt and stamping
	// CreatedAt when it is zero.
	Put(ctx context.Context, user entity.User) (entity.User, error)
	Delete(ctx context.Context, id string) error
}
