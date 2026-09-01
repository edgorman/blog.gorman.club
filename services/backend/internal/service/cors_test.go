package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testOrigin = "https://blog.gorman.club"

func TestWithCORS_NoOriginConfigured(t *testing.T) {
	called := false
	handler := withCORS("", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug", nil))

	if !called {
		t.Fatal("next handler was not called")
	}
	if got := rec.Result().Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestWithCORS_OriginConfigured(t *testing.T) {
	handler := withCORS(testOrigin, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug", nil))

	if got := rec.Result().Header.Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, testOrigin)
	}
}

func TestWithCORS_Preflight(t *testing.T) {
	called := false
	handler := withCORS(testOrigin, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/debug", nil))

	if called {
		t.Fatal("next handler should not be called for OPTIONS preflight")
	}
	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
}

// Routes are registered under a specific method, so ServeMux 405s an OPTIONS preflight before any
// per-route wrapper runs - withCORS must wrap the whole mux to intercept it first.
func TestHandler_PreflightAgainstMethodSpecificMux(t *testing.T) {
	s := New(Config{AllowedOrigin: testOrigin}, newFakeBlogRepository(), newFakeUserRepository(),
		newFakeChatRepository(), newFakeCommentRepository(), newFakeReactionRepository(),
		fakeVerifier{uid: "caller"}, &fakeAssistant{})

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/blogs", nil))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d (preflight must not reach the method-specific mux)", rec.Result().StatusCode, http.StatusNoContent)
	}
}
