package google

import (
	"context"
	"errors"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// An unset client ID is reported distinctly so the service can answer 500 (an operator problem)
// rather than 401, and so no token is ever accepted by a deployment that cannot check its audience.
func TestTokenVerifier_UnconfiguredClientID(t *testing.T) {
	caller, err := NewTokenVerifier("").Verify(context.Background(), "any-token")

	if !errors.Is(err, repository.ErrAuthNotConfigured) {
		t.Fatalf("Verify = %v, want ErrAuthNotConfigured", err)
	}
	if caller.UID != "" {
		t.Errorf("caller = %+v, want the zero value", caller)
	}
}
