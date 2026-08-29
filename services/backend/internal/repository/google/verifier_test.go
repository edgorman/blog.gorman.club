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

// Google has minted email_verified both as a JSON boolean and as the string "true" over the years,
// and anything else at all means the address was not verified. The assistant allowlist is keyed on
// a verified address, so reading this wrongly in the permissive direction would hand access to an
// account that merely claimed one.
func TestEmailVerified(t *testing.T) {
	for _, tt := range []struct {
		name  string
		claim any
		want  bool
	}{
		{"boolean true", true, true},
		{"boolean false", false, false},
		{"string true", "true", true},
		{"string false", "false", false},
		{"absent", nil, false},
		{"unexpected type", 1, false},
		{"unexpected string", "yes", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := emailVerified(tt.claim); got != tt.want {
				t.Errorf("emailVerified(%#v) = %v, want %v", tt.claim, got, tt.want)
			}
		})
	}
}
