package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

func (s *fakeBlogStore) List(ctx context.Context, callerUID string) ([]Blog, error) {
	var out []Blog
	for _, blog := range s.blogs {
		if blog.visibleTo(callerUID) {
			out = append(out, blog)
		}
	}
	return out, nil
}

func (s *fakeBlogStore) Create(ctx context.Context, blog Blog) (Blog, error) {
	s.nextID++
	blog.ID = string(rune('a' + s.nextID))
	s.blogs[blog.ID] = blog
	return blog, nil
}

func (s *fakeBlogStore) Update(ctx context.Context, id string, blog Blog) (Blog, error) {
	if _, ok := s.blogs[id]; !ok {
		return Blog{}, ErrNotFound
	}
	blog.ID = id
	s.blogs[id] = blog
	return blog, nil
}

func (s *fakeBlogStore) Delete(ctx context.Context, id string) error {
	if _, ok := s.blogs[id]; !ok {
		return ErrNotFound
	}
	delete(s.blogs, id)
	return nil
}

func TestBlogHandler_Get_PrivateHiddenFromStranger(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["blog-1"] = Blog{ID: "blog-1", OwnerID: "owner", Visibility: "private"}
	h := newBlogHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/blogs/blog-1", nil)
	req.SetPathValue("id", "blog-1")
	req = withUID(req, "stranger")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d (private blogs must not be distinguishable from missing ones)", rec.Result().StatusCode, http.StatusNotFound)
	}
}

func TestBlogHandler_Get_PublicVisibleToAnyone(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["blog-1"] = Blog{ID: "blog-1", OwnerID: "owner", Visibility: "public", Title: "Hello"}
	h := newBlogHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/blogs/blog-1", nil)
	req.SetPathValue("id", "blog-1")
	req = withUID(req, "stranger")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
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
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
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
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
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
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
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

func TestBlogHandler_List_FiltersToVisible(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["public"] = Blog{ID: "public", Visibility: "public"}
	store.blogs["mine"] = Blog{ID: "mine", OwnerID: "caller", Visibility: "private"}
	store.blogs["others"] = Blog{ID: "others", OwnerID: "someone-else", Visibility: "private"}
	h := newBlogHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/blogs", nil)
	req = withUID(req, "caller")
	rec := httptest.NewRecorder()
	h.List(rec, req)

	var got []Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (public + own private, not others' private)", len(got))
	}
}
