package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// The credential is opaque and the provider header says how to verify it, so adding a provider
// means extending authProvider and the switch in requireAuth.
const (
	authorizationHeader         = "Authorization"
	authorizationProviderHeader = "Authorization-Provider"
	bearerPrefix                = "Bearer "
)

type authProvider string

const providerGoogle authProvider = "google"

type contextKey int

const callerContextKey contextKey = 0

// verifyCredential validates the Authorization/Authorization-Provider pair and returns the
// identity it asserts. It writes the error response and returns ok=false if the credential
// present is malformed, names an unsupported provider, or fails verification. Callers are
// responsible for the case where no credential was supplied at all, since requireAuth and
// optionalAuth treat that differently.
func verifyCredential(verifier repository.TokenVerifier, w http.ResponseWriter, r *http.Request) (entity.Caller, bool) {
	authorization := r.Header.Get(authorizationHeader)

	token, ok := strings.CutPrefix(authorization, bearerPrefix)
	if !ok || token == "" {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("%s header malformed, must start with %q", authorizationHeader, bearerPrefix))
		return entity.Caller{}, false
	}

	provider := r.Header.Get(authorizationProviderHeader)
	if provider == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s is missing", authorizationProviderHeader))
		return entity.Caller{}, false
	}
	if authProvider(provider) != providerGoogle {
		writeError(w, http.StatusNotImplemented,
			fmt.Sprintf("provider %q has not been implemented", provider))
		return entity.Caller{}, false
	}

	caller, err := verifier.Verify(r.Context(), token)
	if errors.Is(err, repository.ErrAuthNotConfigured) {
		// A deployment problem, not the caller's fault.
		writeError(w, http.StatusInternalServerError, "authentication is not configured")
		return entity.Caller{}, false
	}
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return entity.Caller{}, false
	}

	return caller, true
}

func withCaller(r *http.Request, caller entity.Caller) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), callerContextKey, caller))
}

// requireAuth verifies the request's credential and stores the resulting identity in the context,
// so every handler below it can assume the caller has been verified. A malformed request is a 400
// rather than a 401: the caller didn't fail to authenticate, they failed to ask properly.
func requireAuth(verifier repository.TokenVerifier, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(authorizationHeader) == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		caller, ok := verifyCredential(verifier, w, r)
		if !ok {
			return
		}

		next(w, withCaller(r, caller))
	}
}

// optionalAuth verifies the request's credential if one was supplied, for routes that admit
// anonymous callers but still authorize a signed-in one. A request with no Authorization header
// runs unauthenticated, which downstream handlers see as the zero entity.Caller - exactly the
// identity entity.Blog.CanBeReadBy already treats as "not the owner, not on the whitelist". A
// header that is present but invalid is rejected the same way requireAuth rejects it: supplying a
// bad credential is different from supplying none.
func optionalAuth(verifier repository.TokenVerifier, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(authorizationHeader) == "" {
			next(w, r)
			return
		}

		caller, ok := verifyCredential(verifier, w, r)
		if !ok {
			return
		}

		next(w, withCaller(r, caller))
	}
}

func callerFromContext(ctx context.Context) entity.Caller {
	caller, _ := ctx.Value(callerContextKey).(entity.Caller)
	return caller
}

// uidFromContext returns the caller's provider-issued user ID, which is what ownership is
// recorded against.
func uidFromContext(ctx context.Context) string {
	return callerFromContext(ctx).UID
}
