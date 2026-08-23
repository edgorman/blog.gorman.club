package main

import (
	"context"
	"net/http"
	"strings"

	firebaseauth "firebase.google.com/go/v4/auth"
)

type contextKey int

const uidContextKey contextKey = 0

// tokenVerifier checks a Firebase ID token and returns the caller's uid.
type tokenVerifier interface {
	Verify(ctx context.Context, idToken string) (uid string, err error)
}

type firebaseTokenVerifier struct {
	client *firebaseauth.Client
}

func (v *firebaseTokenVerifier) Verify(ctx context.Context, idToken string) (string, error) {
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return "", err
	}
	return token.UID, nil
}

// requireAuth verifies the bearer token in the Authorization header and stores the caller's uid
// in the request context, rejecting the request otherwise. This is what lets handlers enforce
// the same request.auth.uid-based checks firestore.rules defines for direct client access.
func requireAuth(verifier tokenVerifier, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		uid, err := verifier.Verify(r.Context(), token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), uidContextKey, uid)))
	}
}

func uidFromContext(ctx context.Context) string {
	uid, _ := ctx.Value(uidContextKey).(string)
	return uid
}
