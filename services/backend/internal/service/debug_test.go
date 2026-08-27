package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
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

// Every write sits behind requireAuth.
func TestHandler_WriteRoutesRequireAuth(t *testing.T) {
	s := newTestService(nil, nil)

	for _, tt := range []struct{ method, path string }{
		{http.MethodPost, "/blogs"},
		{http.MethodPut, "/blogs/blog-1"},
		{http.MethodDelete, "/blogs/blog-1"},
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

// GET /blogs and GET /blogs/{id} admit anonymous callers - they 401 only for a credential that is
// present but invalid, never merely absent.
func TestHandler_BlogReadRoutesAdmitAnonymousCallers(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["public"] = entity.Blog{ID: "public", OwnerID: "owner", Visibility: entity.VisibilityPublic}
	s := newTestService(repo, nil)

	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, "/blogs"},
		{http.MethodGet, "/blogs/public"},
	} {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Result().StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
			}
		})
	}
}

// An anonymous GET /blogs/{id} for a private post is a 403, the same outcome a signed-in caller
// who isn't the owner or on the whitelist gets - not a 401, since no credential was required.
func TestHandler_AnonymousCallerCannotReadPrivateBlog(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["private"] = entity.Blog{ID: "private", OwnerID: "owner", Visibility: entity.VisibilityPrivate}
	s := newTestService(repo, nil)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/blogs/private", nil))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
}

// GET /users/{id} admits anonymous callers too: a profile has nothing caller-specific to hide.
func TestHandler_UserReadRouteAdmitsAnonymousCallers(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "someone", Username: "quiet-reading-otter", DisplayName: "Someone"})
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/someone", nil))

	if rec.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
}

// An anonymous GET /blogs never surfaces a private post, even one owned by nobody's uid ("").
func TestHandler_AnonymousListOnlyReturnsPublicBlogs(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["public"] = entity.Blog{ID: "public", OwnerID: "owner", Visibility: entity.VisibilityPublic}
	repo.blogs["private"] = entity.Blog{ID: "private", OwnerID: "owner", Visibility: entity.VisibilityPrivate}
	s := newTestService(repo, nil)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/blogs", nil))

	var got []entity.Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 || got[0].ID != "public" {
		t.Errorf("got %v, want only the public blog", got)
	}
}
