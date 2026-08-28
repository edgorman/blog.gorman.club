package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

func blogRequestBody(t *testing.T, body blogRequest) *bytes.Reader {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return bytes.NewReader(encoded)
}

func TestListBlogs_OnlyReadableBlogs(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["public"] = entity.Blog{ID: "public", OwnerID: "someone", Visibility: entity.VisibilityPublic}
	repo.blogs["own"] = entity.Blog{ID: "own", OwnerID: "caller", Visibility: entity.VisibilityPrivate}
	repo.blogs["shared"] = entity.Blog{ID: "shared", OwnerID: "someone", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"caller"}}
	repo.blogs["hidden"] = entity.Blog{ID: "hidden", OwnerID: "someone", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"another"}}
	s := newTestService(repo, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got []entity.Blog
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

func TestListBlogs_NewestFirst(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["older"] = entity.Blog{ID: "older", OwnerID: "caller", Visibility: entity.VisibilityPublic, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	repo.blogs["newer"] = entity.Blog{ID: "newer", OwnerID: "caller", Visibility: entity.VisibilityPublic, CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}
	s := newTestService(repo, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	var got []entity.Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 || got[0].ID != "newer" || got[1].ID != "older" {
		t.Errorf("got %v, want newer before older", got)
	}
}

// An empty collection must serialise as [] so clients can iterate the response unconditionally.
func TestListBlogs_EmptyIsArray(t *testing.T) {
	s := newTestService(nil, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("body = %q, want %q", body, "[]")
	}
}

func TestGetBlog_Readable(t *testing.T) {
	for _, tt := range []struct {
		name string
		blog entity.Blog
	}{
		{"public post", entity.Blog{ID: "blog-1", OwnerID: "someone", Visibility: entity.VisibilityPublic}},
		{"own private post", entity.Blog{ID: "blog-1", OwnerID: "caller", Visibility: entity.VisibilityPrivate}},
		{"whitelisted private post", entity.Blog{ID: "blog-1", OwnerID: "someone", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"caller"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeBlogRepository()
			repo.blogs["blog-1"] = tt.blog
			s := newTestService(repo, nil)

			req := httptest.NewRequest(http.MethodGet, "/blogs/blog-1", nil)
			req.SetPathValue("id", "blog-1")
			rec := httptest.NewRecorder()
			s.GetBlog(rec, withUID(req, "caller"))

			if rec.Result().StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
			}
			var got entity.Blog
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got.ID != "blog-1" {
				t.Errorf("ID = %q, want %q", got.ID, "blog-1")
			}
		})
	}
}

func TestGetBlog_ForbiddenForPrivatePost(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["blog-1"] = entity.Blog{ID: "blog-1", OwnerID: "owner", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"another"}}
	s := newTestService(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/blogs/blog-1", nil)
	req.SetPathValue("id", "blog-1")
	rec := httptest.NewRecorder()
	s.GetBlog(rec, withUID(req, "caller"))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
}

func TestGetBlog_NotFound(t *testing.T) {
	s := newTestService(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/blogs/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	s.GetBlog(rec, withUID(req, "caller"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

// ownerId is not part of blogRequest at all, so a client cannot express one - the handler takes it
// from the verified caller.
func TestCreateBlog_OwnerIDFromCaller(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newTestService(repo, nil)

	body := []byte(`{"title":"New post","visibility":"public","ownerId":"someone-else"}`)
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", bytes.NewReader(body)), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	var got entity.Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.OwnerID != "caller" {
		t.Errorf("OwnerID = %q, want %q (must ignore client-supplied ownerId)", got.OwnerID, "caller")
	}
}

func TestCreateBlog_RejectsInvalidVisibility(t *testing.T) {
	s := newTestService(nil, nil)

	body := blogRequestBody(t, blogRequest{Title: "New post", Visibility: "everyone"})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if got := decodeAPIError(t, rec).Error; !strings.Contains(got, "visibility") {
		t.Errorf("error = %q, want it to name the visibility field", got)
	}
}

func TestCreateBlog_RejectsOverlongTitle(t *testing.T) {
	s := newTestService(nil, nil)

	body := blogRequestBody(t, blogRequest{
		Title:      strings.Repeat("a", entity.MaxTitleLength+1),
		Visibility: entity.VisibilityPublic,
	})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if got := decodeAPIError(t, rec).Error; !strings.Contains(got, "title") {
		t.Errorf("error = %q, want it to name the title field", got)
	}
}

func TestUpdateBlog_ForbiddenForNonOwner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["blog-1"] = entity.Blog{ID: "blog-1", OwnerID: "owner", Visibility: entity.VisibilityPublic}
	s := newTestService(repo, nil)

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: entity.VisibilityPublic})
	req := httptest.NewRequest(http.MethodPut, "/blogs/blog-1", body)
	req.SetPathValue("id", "blog-1")
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "not-the-owner"))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
}

func TestUpdateBlog_NotFound(t *testing.T) {
	s := newTestService(nil, nil)

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: entity.VisibilityPublic})
	req := httptest.NewRequest(http.MethodPut, "/blogs/missing", body)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "caller"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

// A client trying to reassign ownership or backdate the post must not be able to.
func TestUpdateBlog_PreservesOwnerAndCreatedAt(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakeBlogRepository()
	repo.blogs["blog-1"] = entity.Blog{ID: "blog-1", OwnerID: "owner", Visibility: entity.VisibilityPublic, CreatedAt: created}
	s := newTestService(repo, nil)

	body := []byte(`{"title":"Edited","visibility":"public","ownerId":"someone-else","createdAt":"1999-01-01T00:00:00Z"}`)
	req := httptest.NewRequest(http.MethodPut, "/blogs/blog-1", bytes.NewReader(body))
	req.SetPathValue("id", "blog-1")
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored := repo.blogs["blog-1"]
	if stored.OwnerID != "owner" {
		t.Errorf("OwnerID = %q, want %q (must ignore client-supplied ownerId)", stored.OwnerID, "owner")
	}
	if !stored.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v (must be carried over, not taken from the body)", stored.CreatedAt, created)
	}
}

func TestUpdateBlog_Owner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["blog-1"] = entity.Blog{ID: "blog-1", OwnerID: "owner", Visibility: entity.VisibilityPublic, Title: "Original"}
	s := newTestService(repo, nil)

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: entity.VisibilityPrivate})
	req := httptest.NewRequest(http.MethodPut, "/blogs/blog-1", body)
	req.SetPathValue("id", "blog-1")
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if repo.blogs["blog-1"].Title != "Edited" {
		t.Errorf("Title = %q, want %q", repo.blogs["blog-1"].Title, "Edited")
	}
	if repo.blogs["blog-1"].Visibility != entity.VisibilityPrivate {
		t.Errorf("Visibility = %q, want %q", repo.blogs["blog-1"].Visibility, entity.VisibilityPrivate)
	}
}

// An invalid body must not partially apply: the stored blog stays exactly as it was.
func TestUpdateBlog_InvalidBodyLeavesBlogUntouched(t *testing.T) {
	original := entity.Blog{ID: "blog-1", OwnerID: "owner", Visibility: entity.VisibilityPublic, Title: "Original"}
	repo := newFakeBlogRepository()
	repo.blogs["blog-1"] = original
	s := newTestService(repo, nil)

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: "everyone"})
	req := httptest.NewRequest(http.MethodPut, "/blogs/blog-1", body)
	req.SetPathValue("id", "blog-1")
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if !reflect.DeepEqual(repo.blogs["blog-1"], original) {
		t.Errorf("stored = %+v, want it unchanged at %+v", repo.blogs["blog-1"], original)
	}
}

func TestDeleteBlog_ForbiddenForNonOwner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["blog-1"] = entity.Blog{ID: "blog-1", OwnerID: "owner", Visibility: entity.VisibilityPublic}
	s := newTestService(repo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/blogs/blog-1", nil)
	req.SetPathValue("id", "blog-1")
	rec := httptest.NewRecorder()
	s.DeleteBlog(rec, withUID(req, "not-the-owner"))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if _, ok := repo.blogs["blog-1"]; !ok {
		t.Error("blog was deleted despite forbidden caller")
	}
}

func TestDeleteBlog_Owner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["blog-1"] = entity.Blog{ID: "blog-1", OwnerID: "owner", Visibility: entity.VisibilityPublic}
	s := newTestService(repo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/blogs/blog-1", nil)
	req.SetPathValue("id", "blog-1")
	rec := httptest.NewRecorder()
	s.DeleteBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if _, ok := repo.blogs["blog-1"]; ok {
		t.Error("blog still present after delete")
	}
}

// A post carries its author, because a client cannot resolve one: profiles are addressable only by
// username, and a post records its owner by uid.
func TestGetBlog_CarriesTheAuthor(t *testing.T) {
	blogs := newFakeBlogRepository()
	blogs.blogs["b1"] = entity.Blog{ID: "b1", OwnerID: "owner", Visibility: entity.VisibilityPublic}
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "owner", Username: "sly-dancing-monkey", DisplayName: "Ed"})
	s := newTestService(blogs, users)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs/b1", nil), "reader")
	req.SetPathValue("id", "b1")
	rec := httptest.NewRecorder()
	s.GetBlog(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got blogResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Author == nil {
		t.Fatal("Author = nil, want the owner's profile")
	}
	if got.Author.Username != "sly-dancing-monkey" {
		t.Errorf("Author.Username = %q, want %q", got.Author.Username, "sly-dancing-monkey")
	}
	if got.Author.DisplayName != "Ed" {
		t.Errorf("Author.DisplayName = %q, want %q", got.Author.DisplayName, "Ed")
	}
}

// Posting never required a profile, so an owner without one is a null author rather than a failure.
func TestGetBlog_NullAuthorWhenTheOwnerHasNoProfile(t *testing.T) {
	blogs := newFakeBlogRepository()
	blogs.blogs["b1"] = entity.Blog{ID: "b1", OwnerID: "owner", Visibility: entity.VisibilityPublic}
	s := newTestService(blogs, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs/b1", nil), "reader")
	req.SetPathValue("id", "b1")
	rec := httptest.NewRecorder()
	s.GetBlog(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got blogResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Author != nil {
		t.Errorf("Author = %+v, want null", got.Author)
	}
}

// Authors are resolved per distinct owner, not per post, so one author's feed costs one lookup.
func TestListBlogs_ResolvesEachAuthorOnce(t *testing.T) {
	blogs := newFakeBlogRepository()
	for _, id := range []string{"b1", "b2", "b3"} {
		blogs.blogs[id] = entity.Blog{ID: id, OwnerID: "owner", Visibility: entity.VisibilityPublic}
	}
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "owner", Username: "sly-dancing-monkey", DisplayName: "Ed"})
	s := newTestService(blogs, users)

	rec := httptest.NewRecorder()
	s.ListBlogs(rec, withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "reader"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got []blogResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("posts = %d, want 3", len(got))
	}
	for _, post := range got {
		if post.Author == nil || post.Author.Username != "sly-dancing-monkey" {
			t.Fatalf("post %s carries author %+v, want the owner's profile", post.ID, post.Author)
		}
	}
	if users.gets != 1 {
		t.Errorf("user lookups = %d, want 1 for three posts by one author", users.gets)
	}
}
