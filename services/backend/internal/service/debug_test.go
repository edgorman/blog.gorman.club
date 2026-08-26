package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDebug(t *testing.T) {
	s := newTestService(nil, nil)

	rec := httptest.NewRecorder()
	s.Debug(rec, httptest.NewRequest(http.MethodGet, "/debug", nil))

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body debugResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("Status = %q, want %q", body.Status, "ok")
	}
	if body.Environment != "test" {
		t.Errorf("Environment = %q, want %q", body.Environment, "test")
	}
	if body.Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", body.Commit, "abc123")
	}
	if _, err := time.Parse(time.RFC3339, body.Timestamp); err != nil {
		t.Errorf("Timestamp = %q is not RFC3339: %v", body.Timestamp, err)
	}
}

// /health and /debug are the same handler, and neither requires a credential.
func TestHandler_DebugRoutesAreUnauthenticated(t *testing.T) {
	s := newTestService(nil, nil)

	for _, path := range []string{"/health", "/debug"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Result().StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
			}
		})
	}
}

// Every blog and user route sits behind requireAuth.
func TestHandler_DataRoutesRequireAuth(t *testing.T) {
	s := newTestService(nil, nil)

	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, "/blogs"},
		{http.MethodGet, "/blogs/blog-1"},
		{http.MethodPost, "/blogs"},
		{http.MethodPut, "/blogs/blog-1"},
		{http.MethodDelete, "/blogs/blog-1"},
		{http.MethodGet, "/users/caller"},
		{http.MethodPut, "/users/caller"},
		{http.MethodDelete, "/users/caller"},
	} {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Result().StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusUnauthorized)
			}
		})
	}
}
