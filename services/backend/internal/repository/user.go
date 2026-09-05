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
	//
	// It preserves the account's paid access whatever the profile it is handed says, exactly as it
	// preserves CreatedAt: those fields are written by SetSubscription below and by nothing else.
	Put(ctx context.Context, user entity.User) (entity.User, error)
	// SetSubscription records what the payment provider last said about the account's paid access,
	// leaving every other field of the profile alone. It returns ErrNotFound if id holds no
	// profile.
	//
	// It is separate from Put rather than a field a caller could set through it, and that is the
	// point: Put takes a profile a request assembled, so paid access being reachable from there
	// would mean an account could grant itself a subscription by editing its own bio. This is
	// reached only from a verified provider event.
	SetSubscription(ctx context.Context, id string, subscription entity.Subscription) error
	// Delete removes the profile and releases the username it held.
	Delete(ctx context.Context, id string) error
}
