package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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

func (s *fakeBlogStore) List(ctx context.Context, uid string) ([]Blog, error) {
	visible := make([]Blog, 0, len(s.blogs))
	for _, blog := range s.blogs {
		if canRead(blog, uid) {
			visible = append(visible, blog)
		}
	}
	slices.SortFunc(visible, func(a, b Blog) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return visible, nil
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
	return req.WithContext(context.WithValue(req.Context(), callerContextKey, caller{UID: uid}))
}

// decodeAPIError asserts the response carries a JSON error body rather than plain text.
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

func TestBlogHandler_List_OnlyReadableBlogs(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["public"] = Blog{ID: "public", OwnerID: "someone", Visibility: "public"}
	store.blogs["own"] = Blog{ID: "own", OwnerID: "caller", Visibility: "private"}
	store.blogs["shared"] = Blog{ID: "shared", OwnerID: "someone", Visibility: "private", AllowedUserIDs: []string{"caller"}}
	store.blogs["hidden"] = Blog{ID: "hidden", OwnerID: "someone", Visibility: "private", AllowedUserIDs: []string{"another"}}
	h := newBlogHandler(store)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got []Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, blog := range got {
		ids = append(ids, blog.ID)
	}
	slices.Sort(ids)
	want := []string{"own", "public", "shared"}
	if !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}

func TestBlogHandler_List_NewestFirst(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["older"] = Blog{ID: "older", OwnerID: "caller", Visibility: "public", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	store.blogs["newer"] = Blog{ID: "newer", OwnerID: "caller", Visibility: "public", CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	h := newBlogHandler(store)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	h.List(rec, req)

	var got []Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 || got[0].ID != "newer" || got[1].ID != "older" {
		t.Errorf("got %v, want newer before older", got)
	}
}

// An empty collection must serialise as [] so clients can iterate the response unconditionally.
func TestBlogHandler_List_EmptyIsArray(t *testing.T) {
	h := newBlogHandler(newFakeBlogStore())

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %q, want %q", body, "[]")
	}
}

func TestBlogHandler_Get_Readable(t *testing.T) {
	for _, tt := range []struct {
		name string
		blog Blog
	}{
		{"public post", Blog{ID: "blog-1", OwnerID: "someone", Visibility: "public"}},
		{"own private post", Blog{ID: "blog-1", OwnerID: "caller", Visibility: "private"}},
		{"whitelisted private post", Blog{ID: "blog-1", OwnerID: "someone", Visibility: "private", AllowedUserIDs: []string{"caller"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeBlogStore()
			store.blogs["blog-1"] = tt.blog
			h := newBlogHandler(store)

			req := httptest.NewRequest(http.MethodGet, "/blogs/blog-1", nil)
			req.SetPathValue("id", "blog-1")
			rec := httptest.NewRecorder()
			h.Get(rec, withUID(req, "caller"))

			if rec.Result().StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
			}
			var got Blog
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.ID != "blog-1" {
				t.Errorf("ID = %q, want %q", got.ID, "blog-1")
			}
		})
	}
}

func TestBlogHandler_Get_ForbiddenForPrivatePost(t *testing.T) {
	store := newFakeBlogStore()
	store.blogs["blog-1"] = Blog{ID: "blog-1", OwnerID: "owner", Visibility: "private", AllowedUserIDs: []string{"another"}}
	h := newBlogHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/blogs/blog-1", nil)
	req.SetPathValue("id", "blog-1")
	rec := httptest.NewRecorder()
	h.Get(rec, withUID(req, "caller"))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
}

func TestBlogHandler_Get_NotFound(t *testing.T) {
	h := newBlogHandler(newFakeBlogStore())

	req := httptest.NewRequest(http.MethodGet, "/blogs/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	h.Get(rec, withUID(req, "caller"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
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
