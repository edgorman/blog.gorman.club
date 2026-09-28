package repository

import (
	"context"
	"time"

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
	// the stored CreatedAt (stamping it when the profile is new). SubscribedUntil and
	// StripeCustomerID are likewise kept as stored whatever user carries, so no profile write can
	// grant access - SetSubscription is the only way to change them. It rejects a profile that fails
	// entity.User.Validate without writing anything, and returns ErrUsernameTaken - again without
	// writing - if user.Username is already held by somebody else.
	Put(ctx context.Context, user entity.User) (entity.User, error)
	// SetSubscription writes the billing fields of an existing profile and nothing else: until
	// replaces SubscribedUntil (nil clears it) and customerID replaces StripeCustomerID. It
	// returns ErrNotFound if id doesn't exist.
	SetSubscription(ctx context.Context, id, customerID string, until *time.Time) error
	// Delete removes the profile and releases the username it held.
	Delete(ctx context.Context, id string) error
}
