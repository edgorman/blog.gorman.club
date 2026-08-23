package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeBlogStore struct {
	blogs  map[string]Blog
	nextID int
}

func newFakeBlogStore() *fakeBlogStore {
	return &fakeBlogStore{blogs: make(map[string]Blog)}
}

func (s *fakeBlogStore) Get(ctx context.Context, id string) (Blog, error) {
	blog, ok := s.blogs[id]
	if !ok {
		return Blog{}, ErrNotFound
	}
	return blog, nil
}

func (s *fakeBlogStore) Create(ctx context.Context, blog Blog) (Blog, error) {
	s.nextID++
	blog.ID = fmt.Sprintf("blog-%d", s.nextID)
	s.blogs[blog.ID] = blog
	return blog, nil
}

func (s *fakeBlogStore) Update(ctx context.Context, blog Blog) (Blog, error) {
	s.blogs[blog.ID] = blog
	return blog, nil
}

func (s *fakeBlogStore) Delete(ctx context.Context, id string) error {
	if _, ok := s.blogs[id]; !ok {
		return ErrNotFound
	}
	delete(s.blogs, id)
	return nil
}

func withUID(req *http.Request, uid string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), uidContextKey, uid))
}

// decodeAPIError asserts the response carries a JSON error body rather than plain text, so
// clients can parse success and failure the same way.
func decodeAPIError(t *testing.T, rec *httptest.ResponseRecorder) apiError {
	t.Helper()

	if ct := rec.Result().Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	var body apiError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error == "" {
		t.Error("error body has an empty message")
	}
	return body
}

func TestBlogHandler_Create_OwnerIDFromCaller(t *testing.T) {
	store := newFakeBlogStore()
	h := newBlogHandler(store)

	body, _ := json.Marshal(Blog{Title: "New post", Visibility: "public", OwnerID: "someone-else"})
	req := httptest.NewRequest(http.MethodPost, "/blogs", bytes.NewReader(body))
	req = withUID(req, "caller")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	var got Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.OwnerID != "caller" {
		t.Errorf("OwnerID = %q, want %q (must ignore client-supplied ownerId)", got.OwnerID, "caller")
	}
}

func TestBlogHandler_Create_RejectsInvalidVisibility(t *testing.T) {
	h := newBlogHandler(newFakeBlogStore())

	body, _ := json.Marshal(Blog{Title: "New post", Visibility: "everyone"})
	req := httptest.NewRequest(http.MethodPost, "/blogs", bytes.NewReader(body))
	req = withUID(req, "caller")
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	decodeAPIError(t, rec)
}

func TestBlogHandler_Update_ForbiddenForNonOwner(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["blog-1"] = Blog{ID: "blog-1", OwnerID: "owner", Visibility: "public"}
	h := newBlogHandler(store)

	body, _ := json.Marshal(Blog{Title: "Edited", Visibility: "public"})
	req := httptest.NewRequest(http.MethodPut, "/blogs/blog-1", bytes.NewReader(body))
	req.SetPathValue("id", "blog-1")
	req = withUID(req, "not-the-owner")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
}

func TestBlogHandler_Update_NotFound(t *testing.T) {
	h := newBlogHandler(newFakeBlogStore())

	body, _ := json.Marshal(Blog{Title: "Edited", Visibility: "public"})
	req := httptest.NewRequest(http.MethodPut, "/blogs/missing", bytes.NewReader(body))
	req.SetPathValue("id", "missing")
	req = withUID(req, "caller")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

func TestBlogHandler_Update_PreservesOwnerAndCreatedAt(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := newFakeBlogStore()
	store.blogs["blog-1"] = Blog{ID: "blog-1", OwnerID: "owner", Visibility: "public", CreatedAt: created}
	h := newBlogHandler(store)

	// A client trying to reassign ownership or backdate the post must not be able to.
	body, _ := json.Marshal(Blog{Title: "Edited", Visibility: "public", OwnerID: "someone-else", CreatedAt: time.Time{}})
	req := httptest.NewRequest(http.MethodPut, "/blogs/blog-1", bytes.NewReader(body))
	req.SetPathValue("id", "blog-1")
	req = withUID(req, "owner")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored := store.blogs["blog-1"]
	if stored.OwnerID != "owner" {
		t.Errorf("OwnerID = %q, want %q (must ignore client-supplied ownerId)", stored.OwnerID, "owner")
	}
	if !stored.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v (must be carried over, not taken from the body)", stored.CreatedAt, created)
	}
}

func TestBlogHandler_Update_Owner(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["blog-1"] = Blog{ID: "blog-1", OwnerID: "owner", Visibility: "public", Title: "Original"}
	h := newBlogHandler(store)

	body, _ := json.Marshal(Blog{Title: "Edited", Visibility: "private"})
	req := httptest.NewRequest(http.MethodPut, "/blogs/blog-1", bytes.NewReader(body))
	req.SetPathValue("id", "blog-1")
	req = withUID(req, "owner")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if store.blogs["blog-1"].Title != "Edited" {
		t.Errorf("Title = %q, want %q", store.blogs["blog-1"].Title, "Edited")
	}
}

func TestBlogHandler_Delete_ForbiddenForNonOwner(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["blog-1"] = Blog{ID: "blog-1", OwnerID: "owner", Visibility: "public"}
	h := newBlogHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/blogs/blog-1", nil)
	req.SetPathValue("id", "blog-1")
	req = withUID(req, "not-the-owner")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if _, ok := store.blogs["blog-1"]; !ok {
		t.Error("blog was deleted despite forbidden caller")
	}
}

func TestBlogHandler_Delete_Owner(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["blog-1"] = Blog{ID: "blog-1", OwnerID: "owner", Visibility: "public"}
	h := newBlogHandler(store)

	req := httptest.NewRequest(http.MethodDelete, "/blogs/blog-1", nil)
	req.SetPathValue("id", "blog-1")
	req = withUID(req, "owner")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if _, ok := store.blogs["blog-1"]; ok {
		t.Error("blog still present after delete")
	}
}
