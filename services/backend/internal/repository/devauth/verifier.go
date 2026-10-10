// Package devauth signs a caller in from a fixed development token, so the backend can be run
// locally without Google sign-in. cmd/backend wires it only when DEV_AUTH=true and refuses to start
// with that set outside ENVIRONMENT=local, so no deployed environment can accept these tokens.
package devauth

import (
	"context"
	"errors"
	"strings"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// Token is the development token. "dev:<uid>" signs in as another account, so access rules that
// need a second reader (a whitelisted post, someone else's comment) can be tried locally too.
const Token = "dev"

var _ repository.TokenVerifier = TokenVerifier{}

// TokenVerifier implements repository.TokenVerifier for development tokens.
type TokenVerifier struct{}

func (TokenVerifier) Verify(_ context.Context, idToken string) (entity.Caller, error) {
	uid, ok := strings.CutPrefix(idToken, Token+":")
	if idToken == Token {
		uid, ok = Token, true
	}
	if !ok || uid == "" {
		return entity.Caller{}, errors.New("not a development token")
	}
	return entity.Caller{UID: uid, Email: uid + "@localhost", Name: uid, EmailVerified: true}, nil
}
