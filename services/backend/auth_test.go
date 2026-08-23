package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeVerifier struct {
	uid string
	err error
}

func (f fakeVerifier) Verify(ctx context.Context, idToken string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.uid, nil
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	called := false
	handler := requireAuth(fakeVerifier{uid: "user-1"}, func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/blogs", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if called {
		t.Fatal("next handler should not be called without a bearer token")
	}
	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusUnauthorized)
	}
	decodeAPIError(t, rec)
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	handler := requireAuth(fakeVerifier{err: errors.New("bad token")}, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called for an invalid token")
	})

	req := httptest.NewRequest(http.MethodGet, "/blogs", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusUnauthorized)
	}
	decodeAPIError(t, rec)
}

func TestRequireAuth_Valid(t *testing.T) {
	var gotUID string
	handler := requireAuth(fakeVerifier{uid: "user-1"}, func(w http.ResponseWriter, r *http.Request) {
		gotUID = uidFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/blogs", nil)
	req.Header.Set("Authorization", "Bearer good-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if gotUID != "user-1" {
		t.Errorf("uidFromContext = %q, want %q", gotUID, "user-1")
	}
}
