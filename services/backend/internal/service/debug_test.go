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
		{http.MethodPut, "/blogs/hello-world"},
		{http.MethodDelete, "/blogs/hello-world"},
		// Reading a thread is anonymous (below), but writing one is not: a comment is signed by
		// whoever wrote it, and deleting one is decided against that signature.
		{http.MethodPost, "/blogs/hello-world/comments"},
		{http.MethodDelete, "/blogs/hello-world/comments/c1"},
		// Reading a bar is anonymous (below); being counted in one is not, since a reaction is one
		// reader counted once and there is no way to mean that without knowing who they are.
		{http.MethodPut, "/blogs/hello-world/reactions/%F0%9F%91%8D"},
		{http.MethodDelete, "/blogs/hello-world/reactions/%F0%9F%91%8D"},
		{http.MethodPut, "/blogs/hello-world/comments/c1/reactions/%F0%9F%91%8D"},
		{http.MethodDelete, "/blogs/hello-world/comments/c1/reactions/%F0%9F%91%8D"},
		{http.MethodGet, "/users/me"},
		{http.MethodPut, "/users/me"},
		{http.MethodDelete, "/users/me"},
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

// GET /blogs, GET /blogs/{slug} and a public post's comments and reactions admit anonymous callers - they 401
// only for a credential that is present but invalid, never merely absent.
func TestHandler_BlogReadRoutesAdmitAnonymousCallers(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "public", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, "/blogs"},
		{http.MethodGet, "/blogs/public"},
		{http.MethodGet, "/blogs/public/comments"},
		{http.MethodGet, "/blogs/public/reactions"},
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

// An anonymous GET /blogs/{slug} for a private post is a 404, the same outcome a signed-in caller
// who isn't the owner or on the whitelist gets - not a 401, since no credential was required, and
// not a 403, since the address is guessable from the post's own title.
func TestHandler_AnonymousCallerCannotReadPrivateBlog(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "private", OwnerID: "owner", Visibility: entity.VisibilityPrivate})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/blogs/private", nil))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
}

// GET /users/{username} admits anonymous callers too: a profile has nothing caller-specific to
// hide, and a username is public by nature.
func TestHandler_UserReadRouteAdmitsAnonymousCallers(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "someone", Username: "quiet-reading-otter"})
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/quiet-reading-otter", nil))

	if rec.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
}

// An anonymous GET /blogs never surfaces a private post, even one owned by nobody's uid ("").
func TestHandler_AnonymousListOnlyReturnsPublicBlogs(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "public", OwnerID: "owner", Visibility: entity.VisibilityPublic},
		entity.Blog{Slug: "private", OwnerID: "owner", Visibility: entity.VisibilityPrivate},
	)
	s := newTestService(repo, nil)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/blogs", nil))

	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Posts) != 1 || got.Posts[0].Slug != "public" {
		t.Errorf("got %v, want only the public blog", got.Posts)
	}
}
