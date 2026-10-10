package main

import (
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/devauth"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository/google"
)

// The development verifier signs anybody in as anybody, so every environment but local must refuse
// to start with it - including the unset ENVIRONMENT a misconfigured deployment would have.
func TestTokenVerifier_DevAuthOnlyLocal(t *testing.T) {
	for _, environment := range []string{"development", "stag", "staging", "prod", "Local", ""} {
		if verifier, err := tokenVerifier(environment, true, "client"); err == nil {
			t.Errorf("tokenVerifier(%q, devAuth) = %T, want an error", environment, verifier)
		}
	}

	verifier, err := tokenVerifier("local", true, "")
	if _, ok := verifier.(devauth.TokenVerifier); err != nil || !ok {
		t.Errorf("tokenVerifier(local, devAuth) = %T, %v; want the development verifier", verifier, err)
	}
	for _, environment := range []string{"local", "prod"} {
		verifier, err := tokenVerifier(environment, false, "client")
		if _, ok := verifier.(*google.TokenVerifier); err != nil || !ok {
			t.Errorf("tokenVerifier(%q) = %T, %v; want the Google verifier", environment, verifier, err)
		}
	}
}
