package service

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
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

// A post is addressed by its title, so the first one to use a title lands at the plain slug of it.
func TestCreateBlog_IDFromTitle(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newTestService(repo, nil)

	body := blogRequestBody(t, blogRequest{Title: "Hello, world!", Visibility: entity.VisibilityPublic})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	var got entity.Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "hello-world" {
		t.Errorf("ID = %q, want %q", got.ID, "hello-world")
	}
	if _, stored := repo.blogs["hello-world"]; !stored {
		t.Errorf("stored under %v, want the post written at its title's slug", slices.Collect(maps.Keys(repo.blogs)))
	}
}

// A title nobody can slug is still a post that needs an address, and two of them still need
// telling apart - which is the same fallback a shared title takes.
func TestCreateBlog_UntitledPostsAreStillAddressable(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newTestService(repo, nil)

	ids := make([]string, 0, 2)
	for range 2 {
		body := blogRequestBody(t, blogRequest{Visibility: entity.VisibilityPublic})
		req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
		rec := httptest.NewRecorder()
		s.CreateBlog(rec, req)

		if rec.Result().StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
		}
		var got entity.Blog
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		ids = append(ids, got.ID)
	}

	if ids[0] != "untitled" {
		t.Errorf("first ID = %q, want %q", ids[0], "untitled")
	}
	if !strings.HasPrefix(ids[1], "untitled-") || ids[1] == ids[0] {
		t.Errorf("second ID = %q, want a suffixed %q", ids[1], "untitled")
	}
}

// The second post to use a title cannot have the plain slug, so it takes a suffixed one rather
// than failing or overwriting the post already there.
func TestCreateBlog_SuffixesATakenTitle(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newTestService(repo, nil)

	create := func() entity.Blog {
		t.Helper()

		body := blogRequestBody(t, blogRequest{Title: "Hello world", Content: "Body", Visibility: entity.VisibilityPublic})
		req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
		rec := httptest.NewRecorder()
		s.CreateBlog(rec, req)

		if rec.Result().StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
		}
		var got entity.Blog
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return got
	}

	first, second := create(), create()

	if first.ID != "hello-world" {
		t.Errorf("first ID = %q, want %q", first.ID, "hello-world")
	}
	if !strings.HasPrefix(second.ID, "hello-world-") {
		t.Errorf("second ID = %q, want it to keep the title and add a suffix", second.ID)
	}
	if suffix := strings.TrimPrefix(second.ID, "hello-world-"); len(suffix) != 5 {
		t.Errorf("suffix of %q is %d characters, want 5", second.ID, len(suffix))
	}
	// The first post must still be where it was: a colliding post takes a new id, it does not
	// displace the one that got there first.
	if stored := repo.blogs["hello-world"]; stored.CreatedAt != first.CreatedAt {
		t.Errorf("post at %q = %+v, want the first post left untouched", "hello-world", stored)
	}
	if len(repo.blogs) != 2 {
		t.Errorf("stored %d posts, want both to survive", len(repo.blogs))
	}
}

// Running out of draws is a server failure, not a bad request: the client never chose an id, so
// there is nothing for it to correct.
func TestCreateBlog_FailsWhenNoIDIsFree(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.beforeCreate = func(entity.Blog) error { return repository.ErrBlogIDTaken }
	s := newTestService(repo, nil)

	body := blogRequestBody(t, blogRequest{Title: "Hello world", Visibility: entity.VisibilityPublic})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusInternalServerError)
	}
	decodeAPIError(t, rec)
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
// An id is assigned once, from the title the post was created with. Re-deriving it on every edit
// would break every link to the post and free its old id for somebody else to take, so a retitled
// post keeps the address it has always had.
func TestUpdateBlog_RetitlingKeepsTheID(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.blogs["hello-world"] = entity.Blog{ID: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic, Title: "Hello world"}
	s := newTestService(repo, nil)

	body := blogRequestBody(t, blogRequest{Title: "Something else entirely", Visibility: entity.VisibilityPublic})
	req := withUID(httptest.NewRequest(http.MethodPut, "/blogs/hello-world", body), "owner")
	req.SetPathValue("id", "hello-world")
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored, ok := repo.blogs["hello-world"]
	if !ok {
		t.Fatalf("stored under %v, want the post left at its original id", slices.Collect(maps.Keys(repo.blogs)))
	}
	if stored.Title != "Something else entirely" {
		t.Errorf("Title = %q, want the edit applied", stored.Title)
	}
	if len(repo.blogs) != 1 {
		t.Errorf("stored %d posts, want the edit not to have created a second", len(repo.blogs))
	}
}

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

// A post carries its author's username, because a client cannot resolve one: profiles are
// addressable only by username, and a post records its owner by uid.
func TestGetBlog_CarriesTheAuthor(t *testing.T) {
	blogs := newFakeBlogRepository()
	blogs.blogs["b1"] = entity.Blog{ID: "b1", OwnerID: "owner", Visibility: entity.VisibilityPublic}
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "owner", Username: "sly-dancing-monkey"})
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
	if got.AuthorUsername != "sly-dancing-monkey" {
		t.Errorf("AuthorUsername = %q, want %q", got.AuthorUsername, "sly-dancing-monkey")
	}
}

// Posting never required a profile, so an owner without one is an empty author, not a failure.
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
	if got.AuthorUsername != "" {
		t.Errorf("AuthorUsername = %q, want it empty", got.AuthorUsername)
	}
}

// Authors are resolved per distinct owner, not per post, so one author's feed costs one lookup.
func TestListBlogs_ResolvesEachAuthorOnce(t *testing.T) {
	blogs := newFakeBlogRepository()
	for _, id := range []string{"b1", "b2", "b3"} {
		blogs.blogs[id] = entity.Blog{ID: id, OwnerID: "owner", Visibility: entity.VisibilityPublic}
	}
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "owner", Username: "sly-dancing-monkey"})
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
		if post.AuthorUsername != "sly-dancing-monkey" {
			t.Fatalf("post %s carries author %q, want the owner's username", post.ID, post.AuthorUsername)
		}
	}
	if users.gets != 1 {
		t.Errorf("user lookups = %d, want 1 for three posts by one author", users.gets)
	}
}
