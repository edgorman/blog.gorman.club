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

func (f fakeVerifier) Verify(ctx context.Context, idToken string) (caller, error) {
	if f.err != nil {
		return caller{}, f.err
	}
	return caller{UID: f.uid, Email: "user@example.com", Name: "User"}, nil
}

// authedRequest builds a request carrying both headers a verified call needs.
func authedRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/blogs", nil)
	req.Header.Set(authorizationHeader, bearerPrefix+token)
	req.Header.Set(authorizationProviderHeader, string(providerGoogle))
	return req
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

	rec := httptest.NewRecorder()
	handler(rec, authedRequest("bad-token"))

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

	rec := httptest.NewRecorder()
	handler(rec, authedRequest("good-token"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if gotUID != "user-1" {
		t.Errorf("uidFromContext = %q, want %q", gotUID, "user-1")
	}
}

// Neither an unnamed nor an unsupported provider is a failed credential, so neither is a 401.
func TestRequireAuth_MissingProviderHeader(t *testing.T) {
	handler := requireAuth(fakeVerifier{uid: "user-1"}, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called without a provider header")
	})

	req := httptest.NewRequest(http.MethodGet, "/blogs", nil)
	req.Header.Set(authorizationHeader, bearerPrefix+"good-token")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	decodeAPIError(t, rec)
}

func TestRequireAuth_UnsupportedProvider(t *testing.T) {
	handler := requireAuth(fakeVerifier{uid: "user-1"}, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called for an unsupported provider")
	})

	req := httptest.NewRequest(http.MethodGet, "/blogs", nil)
	req.Header.Set(authorizationHeader, bearerPrefix+"good-token")
	req.Header.Set(authorizationProviderHeader, "facebook")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Result().StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotImplemented)
	}
	decodeAPIError(t, rec)
}

func TestRequireAuth_MalformedAuthorizationHeader(t *testing.T) {
	handler := requireAuth(fakeVerifier{uid: "user-1"}, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called for a malformed header")
	})

	req := httptest.NewRequest(http.MethodGet, "/blogs", nil)
	req.Header.Set(authorizationHeader, "Basic dXNlcjpwYXNz")
	req.Header.Set(authorizationProviderHeader, string(providerGoogle))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	decodeAPIError(t, rec)
}

// An unconfigured deployment is a 500: the operator is at fault, not the caller.
func TestRequireAuth_UnconfiguredIsServerError(t *testing.T) {
	handler := requireAuth(&googleTokenVerifier{clientID: ""}, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called when auth is unconfigured")
	})

	rec := httptest.NewRecorder()
	handler(rec, authedRequest("any-token"))

	if rec.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusInternalServerError)
	}
	decodeAPIError(t, rec)
}

// The identity handlers authorize against must come from the verified token, not the request.
func TestRequireAuth_CallerComesFromVerifiedToken(t *testing.T) {
	var got caller
	handler := requireAuth(fakeVerifier{uid: "user-1"}, func(w http.ResponseWriter, r *http.Request) {
		got = callerFromContext(r.Context())
	})

	rec := httptest.NewRecorder()
	handler(rec, authedRequest("good-token"))

	if got.UID != "user-1" || got.Email != "user@example.com" || got.Name != "User" {
		t.Errorf("caller = %+v, want the verifier's payload", got)
	}
}
