package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCORS_NoOriginConfigured(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "")

	called := false
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/debug", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("next handler was not called")
	}
	if got := rec.Result().Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestWithCORS_OriginConfigured(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://blog.gorman.club")

	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/debug", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Result().Header.Get("Access-Control-Allow-Origin"); got != "https://blog.gorman.club" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://blog.gorman.club")
	}
}

func TestWithCORS_Preflight(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://blog.gorman.club")

	called := false
	handler := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/debug", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("next handler should not be called for OPTIONS preflight")
	}
	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
}

// Routes are registered under a specific method, so ServeMux 405s an OPTIONS preflight before any
// per-route wrapper runs - withCORS must wrap the whole mux to intercept it first.
func TestWithCORS_PreflightAgainstMethodSpecificMux(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://blog.gorman.club")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /blogs", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("POST /blogs", func(w http.ResponseWriter, r *http.Request) {})

	handler := withCORS(mux)

	req := httptest.NewRequest(http.MethodOptions, "/blogs", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d (preflight must not reach the method-specific mux)", rec.Result().StatusCode, http.StatusNoContent)
	}
}
