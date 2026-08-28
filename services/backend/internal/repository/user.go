package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// UserRepository persists user profiles.
type UserRepository interface {
	// Get returns ErrNotFound if id doesn't exist.
	Get(ctx context.Context, id string) (entity.User, error)
	// GetByUsername returns the profile holding username, or ErrNotFound if no profile holds it.
	// The lookup ignores case, matching how uniqueness is enforced.
	GetByUsername(ctx context.Context, username string) (entity.User, error)
	// Put writes the record at user.ID, creating it if absent, refreshing UpdatedAt and preserving
	// the stored CreatedAt (stamping it when the profile is new). It rejects a profile that fails
	// entity.User.Validate without writing anything, and returns ErrUsernameTaken - again without
	// writing - if user.Username is already held by somebody else.
	Put(ctx context.Context, user entity.User) (entity.User, error)
	// Delete removes the profile and releases the username it held.
	Delete(ctx context.Context, id string) error
}
