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

// requireAuth verifies the request's credential and stores the resulting identity in the context,
// so every handler below it can assume the caller has been verified. A malformed request is a 400
// rather than a 401: the caller didn't fail to authenticate, they failed to ask properly.
func requireAuth(verifier repository.TokenVerifier, next http.HandlerFunc) http.HandlerFunc {
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

		caller, err := verifier.Verify(r.Context(), token)
		if errors.Is(err, repository.ErrAuthNotConfigured) {
			// A deployment problem, not the caller's fault.
			writeError(w, http.StatusInternalServerError, "authentication is not configured")
			return
		}
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), callerContextKey, caller)))
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
