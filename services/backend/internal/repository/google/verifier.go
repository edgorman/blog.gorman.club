// Package google verifies Google Identity Services credentials against Google's public keys.
package google

import (
	"context"
	"fmt"
	"slices"

	"google.golang.org/api/idtoken"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// Google mints ID tokens with either issuer.
var issuers = []string{"accounts.google.com", "https://accounts.google.com"}

var _ repository.TokenVerifier = (*TokenVerifier)(nil)

// TokenVerifier implements repository.TokenVerifier. clientID is the OAuth 2.0 client ID the
// frontend signs in with; a token minted for a different client is not accepted.
type TokenVerifier struct {
	clientID string
}

func NewTokenVerifier(clientID string) *TokenVerifier {
	return &TokenVerifier{clientID: clientID}
}

func (v *TokenVerifier) Verify(ctx context.Context, idToken string) (entity.Caller, error) {
	if v.clientID == "" {
		return entity.Caller{}, repository.ErrAuthNotConfigured
	}

	// Validate checks the signature, expiry, and audience; the issuer is checked below.
	payload, err := idtoken.Validate(ctx, idToken, v.clientID)
	if err != nil {
		return entity.Caller{}, err
	}
	if !slices.Contains(issuers, payload.Issuer) {
		return entity.Caller{}, fmt.Errorf("unexpected issuer %q", payload.Issuer)
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	if name == "" {
		name = email
	}
	return entity.Caller{
		UID:           payload.Subject,
		Email:         email,
		Name:          name,
		EmailVerified: emailVerified(payload.Claims["email_verified"]),
	}, nil
}

// emailVerified reads the email_verified claim, which Google has minted both as a JSON boolean and
// as the string "true" over the years. Anything else - absent, malformed, false - is not verified,
// which is the safe way to be wrong about it.
func emailVerified(claim any) bool {
	switch value := claim.(type) {
	case bool:
		return value
	case string:
		return value == "true"
	default:
		return false
	}
}
