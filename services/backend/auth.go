package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/api/idtoken"
)

// Header names and the provider dispatch value, following the Authorization /
// Authorization-Provider convention: the credential itself is opaque, and the provider header
// says how to verify it. Adding a second provider means extending authProvider and the switch in
// requireAuth, not restructuring the flow.
const (
	authorizationHeader         = "Authorization"
	authorizationProviderHeader = "Authorization-Provider"
	bearerPrefix                = "Bearer "
)

type authProvider string

const providerGoogle authProvider = "google"

type contextKey int

const callerContextKey contextKey = 0

// caller is the verified identity behind a request. Every field comes from the provider's signed
// token payload, never from anything else the client sent.
type caller struct {
	UID   string
	Email string
	Name  string
}

// tokenVerifier checks a provider-issued ID token and returns the identity it asserts.
type tokenVerifier interface {
	Verify(ctx context.Context, idToken string) (caller, error)
}

// googleTokenVerifier validates Google Identity Services credentials: signature against Google's
// public keys, plus audience and issuer. clientID is the OAuth 2.0 client ID the frontend signs
// in with - a token minted for a different client must not be accepted here.
type googleTokenVerifier struct {
	clientID string
}

// Google mints ID tokens with either issuer, so both are valid.
var googleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

var errAuthNotConfigured = errors.New("google authentication is not configured")

func (v *googleTokenVerifier) Verify(ctx context.Context, token string) (caller, error) {
	if v.clientID == "" {
		return caller{}, errAuthNotConfigured
	}

	// Validate checks the signature, expiry, and audience; the issuer is checked below.
	payload, err := idtoken.Validate(ctx, token, v.clientID)
	if err != nil {
		return caller{}, err
	}
	if !contains(googleIssuers, payload.Issuer) {
		return caller{}, fmt.Errorf("unexpected issuer %q", payload.Issuer)
	}

	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	if name == "" {
		name = email
	}
	return caller{UID: payload.Subject, Email: email, Name: name}, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// requireAuth verifies the credential on the request and stores the resulting identity in the
// context, rejecting the request otherwise. Handlers authorize against that identity, so every
// route below it can assume the caller has been verified.
//
// A malformed request (missing or unparseable headers) is a 400 rather than a 401: the caller
// didn't fail to authenticate, they failed to ask properly.
func requireAuth(verifier tokenVerifier, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get(authorizationHeader)
		if authorization == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		token, ok := strings.CutPrefix(authorization, bearerPrefix)
		if !ok || token == "" {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("%s header malformed, must start with %q", authorizationHeader, bearerPrefix))
			return
		}

		provider := r.Header.Get(authorizationProviderHeader)
		if provider == "" {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("%s is missing", authorizationProviderHeader))
			return
		}
		if authProvider(provider) != providerGoogle {
			writeError(w, http.StatusNotImplemented,
				fmt.Sprintf("provider %q has not been implemented", provider))
			return
		}

		identity, err := verifier.Verify(r.Context(), token)
		if errors.Is(err, errAuthNotConfigured) {
			// A deployment problem, not the caller's fault.
			writeError(w, http.StatusInternalServerError, "authentication is not configured")
			return
		}
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), callerContextKey, identity)))
	}
}

func callerFromContext(ctx context.Context) caller {
	identity, _ := ctx.Value(callerContextKey).(caller)
	return identity
}

// uidFromContext returns just the caller's stable provider-issued user ID, which is what
// ownership is recorded against.
func uidFromContext(ctx context.Context) string {
	return callerFromContext(ctx).UID
}
