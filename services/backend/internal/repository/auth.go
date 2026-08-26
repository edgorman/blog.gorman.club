package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// TokenVerifier checks a provider-issued ID token with that provider and returns the identity it
// asserts, or ErrAuthNotConfigured if the deployment can't verify anything.
type TokenVerifier interface {
	Verify(ctx context.Context, idToken string) (entity.Caller, error)
}
