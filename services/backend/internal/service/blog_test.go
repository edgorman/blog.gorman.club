package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// blogPathRequest addresses a post the way its route does: by its slug, alone. The target is
// escaped but the path value is not, since that is what a real request produces - ServeMux decodes
// each segment before a handler reads it.
func blogPathRequest(method, slug string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, "/blogs/"+url.PathEscape(slug), body)
	req.SetPathValue("slug", slug)
	return req
}

// newBlogService wires a service over posts and the profiles behind their owners. The address no
// longer runs through a profile, but a response still reports its author by username, so seeding
// one is what lets a test assert on the author a post comes back with.
func newBlogService(blogs repository.BlogRepository, authors ...entity.User) *Service {
	users := newFakeUserRepository()
	for _, author := range authors {
		users.seed(author)
	}
	return newTestService(blogs, users)
}

// author is the profile behind a post's owner uid, named as a response would report it.
func author(uid, username string) entity.User {
	return entity.User{ID: uid, Username: username}
}

func decodeBlog(t *testing.T, rec *httptest.ResponseRecorder) blogResponse {
	t.Helper()

	var got blogResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return got
}

func TestListBlogs_OnlyReadableBlogs(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "public", OwnerID: "someone", Visibility: entity.VisibilityPublic},
		entity.Blog{Slug: "own", OwnerID: "caller", Visibility: entity.VisibilityPrivate},
		entity.Blog{Slug: "shared", OwnerID: "someone", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"caller"}},
		entity.Blog{Slug: "hidden", OwnerID: "someone", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"another"}},
	)
	s := newTestService(repo, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	slugs := make([]string, 0, len(got.Posts))
	for _, blog := range got.Posts {
		slugs = append(slugs, blog.Slug)
	}
	slices.Sort(slugs)
	want := []string{"own", "public", "shared"}
	if !slices.Equal(slugs, want) {
		t.Errorf("slugs = %v, want %v", slugs, want)
	}
}

func TestListBlogs_NewestFirst(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "older", OwnerID: "caller", Visibility: entity.VisibilityPublic, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		entity.Blog{Slug: "newer", OwnerID: "caller", Visibility: entity.VisibilityPublic, CreatedAt: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	)
	s := newTestService(repo, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Posts) != 2 || got.Posts[0].Slug != "newer" || got.Posts[1].Slug != "older" {
		t.Errorf("got %v, want newer before older", got.Posts)
	}
}

// An empty page's posts must serialise as [] so clients can iterate the response unconditionally.
func TestListBlogs_EmptyIsArray(t *testing.T) {
	s := newTestService(nil, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "caller")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body, err := json.Marshal(got.Posts); err != nil || string(body) != "[]" {
		t.Errorf("posts = %s, want %q", body, "[]")
	}
	if got.HasMore {
		t.Errorf("hasMore = true, want false")
	}
}

// The page size is capped at the caller's `limit`, and hasMore says whether a further page exists
// - the two things a client needs to drive "load more" without ever fetching the whole feed.
func TestListBlogs_Paginates(t *testing.T) {
	repo := newFakeBlogRepository()
	for i, day := range []int{1, 2, 3} {
		repo.seed(entity.Blog{
			Slug:       fmt.Sprintf("post-%d", i),
			OwnerID:    "caller",
			Visibility: entity.VisibilityPublic,
			CreatedAt:  time.Date(2026, 1, day, 0, 0, 0, 0, time.UTC),
		})
	}
	s := newTestService(repo, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs?limit=2", nil), "caller")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	var page1 blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&page1); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(page1.Posts) != 2 || page1.Posts[0].Slug != "post-2" || page1.Posts[1].Slug != "post-1" {
		t.Fatalf("page1 = %v, want [post-2 post-1]", page1.Posts)
	}
	if !page1.HasMore {
		t.Fatalf("page1.HasMore = false, want true")
	}

	cursor := page1.Posts[len(page1.Posts)-1].CreatedAt.Format(time.RFC3339Nano)
	req2 := withUID(httptest.NewRequest(http.MethodGet, "/blogs?limit=2&startAfter="+url.QueryEscape(cursor), nil), "caller")
	rec2 := httptest.NewRecorder()
	s.ListBlogs(rec2, req2)

	var page2 blogListResponse
	if err := json.NewDecoder(rec2.Body).Decode(&page2); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(page2.Posts) != 1 || page2.Posts[0].Slug != "post-0" {
		t.Fatalf("page2 = %v, want [post-0]", page2.Posts)
	}
	if page2.HasMore {
		t.Fatalf("page2.HasMore = true, want false")
	}
}

// A malformed limit or startAfter is a 400, not a value silently ignored.
func TestListBlogs_RejectsMalformedParams(t *testing.T) {
	for _, tt := range []struct {
		name  string
		query string
	}{
		{"non-numeric limit", "limit=abc"},
		{"negative limit", "limit=-1"},
		{"non-timestamp startAfter", "startAfter=not-a-time"},
		// A tag that normalizes away was still asked for, so answering the whole feed to it
		// would silently ignore the filter.
		{"tag that names nothing", "tag=%21%21%21"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestService(nil, nil)
			req := withUID(httptest.NewRequest(http.MethodGet, "/blogs?"+tt.query, nil), "caller")
			rec := httptest.NewRecorder()
			s.ListBlogs(rec, req)

			if rec.Result().StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// `ownerId` narrows a page to one author's posts - what a profile feed asks for - and still hides
// what that viewer may not read, exactly as the general feed does.
func TestListBlogs_OwnerIDScopesToOneAuthor(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "mine-public", OwnerID: "author", Visibility: entity.VisibilityPublic},
		entity.Blog{Slug: "mine-private", OwnerID: "author", Visibility: entity.VisibilityPrivate},
		entity.Blog{Slug: "someone-elses", OwnerID: "someone", Visibility: entity.VisibilityPublic},
	)
	s := newTestService(repo, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs?ownerId=author", nil), "reader")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Posts) != 1 || got.Posts[0].Slug != "mine-public" {
		t.Errorf("posts = %v, want [mine-public]", got.Posts)
	}
}

func TestGetBlog_Readable(t *testing.T) {
	for _, tt := range []struct {
		name string
		blog entity.Blog
	}{
		{"public post", entity.Blog{Slug: "hello-world", OwnerID: "someone", Visibility: entity.VisibilityPublic}},
		{"own private post", entity.Blog{Slug: "hello-world", OwnerID: "caller", Visibility: entity.VisibilityPrivate}},
		{"whitelisted private post", entity.Blog{Slug: "hello-world", OwnerID: "someone", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"caller"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeBlogRepository()
			repo.seed(tt.blog)
			s := newBlogService(repo, author(tt.blog.OwnerID, "sly-dancing-monkey"))

			req := blogPathRequest(http.MethodGet, "hello-world", nil)
			rec := httptest.NewRecorder()
			s.GetBlog(rec, withUID(req, "caller"))

			if rec.Result().StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
			}
			if got := decodeBlog(t, rec); got.Slug != "hello-world" {
				t.Errorf("Slug = %q, want %q", got.Slug, "hello-world")
			}
		})
	}
}

// A slug names at most one post anywhere, so a reader following one reaches it without naming an
// author - which is the whole point of making slugs unique globally rather than per author.
func TestGetBlog_ResolvesTheSlugWithoutAnAuthor(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "hello-world", OwnerID: "first", Title: "First author", Visibility: entity.VisibilityPublic},
		entity.Blog{Slug: "hello-world-k3m9x", OwnerID: "second", Title: "Second author", Visibility: entity.VisibilityPublic},
	)
	s := newBlogService(repo, author("first", "sly-dancing-monkey"), author("second", "bold-leaping-otter"))

	for slug, want := range map[string]string{
		"hello-world":       "First author",
		"hello-world-k3m9x": "Second author",
	} {
		req := blogPathRequest(http.MethodGet, slug, nil)
		rec := httptest.NewRecorder()
		s.GetBlog(rec, withUID(req, "reader"))

		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
		}
		if got := decodeBlog(t, rec); got.Title != want {
			t.Errorf("/blogs/%s = %q, want %q", slug, got.Title, want)
		}
	}
}

// A caller who cannot read a private post is shown the same 404 a missing post gets, not a 403:
// the address is a function of the post's own title, so a 403 would let a stranger confirm the
// post exists just by guessing at it.
func TestGetBlog_NotFoundForUnreadablePrivatePost(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"another"}})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodGet, "hello-world", nil)
	rec := httptest.NewRecorder()
	s.GetBlog(rec, withUID(req, "caller"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

func TestGetBlog_NotFound(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodGet, "missing", nil)
	rec := httptest.NewRecorder()
	s.GetBlog(rec, withUID(req, "caller"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

// A malformed address gets the same 404 a well-formed but absent one does, rather than a 400
// naming the rule it broke: the path is a URL a reader followed, and the shape of a rejected slug
// would tell a prober as much about what exists as an outright miss would. A slug the frontend
// reserves for a route of its own is refused the same way, since no post can hold one.
func TestGetBlog_NotFoundForMalformedAddresses(t *testing.T) {
	for _, tt := range []struct{ name, slug string }{
		{"bad slug", "Hello World"},
		{"empty slug", ""},
		{"reserved slug", "new"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newBlogService(newFakeBlogRepository(), author("owner", "sly-dancing-monkey"))

			req := blogPathRequest(http.MethodGet, tt.slug, nil)
			rec := httptest.NewRecorder()
			s.GetBlog(rec, withUID(req, "caller"))

			if rec.Result().StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
			}
			decodeAPIError(t, rec)
		})
	}
}

// ownerId is not part of blogRequest at all, so a client cannot express one - the handler takes it
// from the verified caller.
func TestCreateBlog_OwnerIDFromCaller(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newBlogService(repo, author("caller", "sly-dancing-monkey"))

	body := []byte(`{"title":"New post","visibility":"public","ownerId":"someone-else"}`)
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", bytes.NewReader(body)), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	if got := decodeBlog(t, rec); got.OwnerID != "caller" {
		t.Errorf("OwnerID = %q, want %q (must ignore client-supplied ownerId)", got.OwnerID, "caller")
	}
}

// A post is addressed by its slug alone, so the response carries the slug the server chose; the
// author comes back beside it because a client cannot resolve one for itself.
func TestCreateBlog_SlugFromTitle(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newBlogService(repo, author("caller", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Hello, world!", Visibility: entity.VisibilityPublic})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	got := decodeBlog(t, rec)
	if got.Slug != "hello-world" {
		t.Errorf("Slug = %q, want %q", got.Slug, "hello-world")
	}
	if got.AuthorUsername != "sly-dancing-monkey" {
		t.Errorf("AuthorUsername = %q, want the post attributed to its author", got.AuthorUsername)
	}
	if _, stored := repo.stored("hello-world"); !stored {
		t.Errorf("stored under %v, want the post written at its slug", slices.Collect(maps.Keys(repo.blogs)))
	}
}

// Slugs are unique across every author, so a title one person has used is taken for everyone else
// too - the second post under it is suffixed whoever wrote it. That is what lets the author be
// left out of the address.
func TestCreateBlog_SlugsAreUniqueAcrossAuthors(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newBlogService(repo, author("first", "sly-dancing-monkey"), author("second", "bold-leaping-otter"))

	slugs := make(map[string]string, 2)
	for _, uid := range []string{"first", "second"} {
		body := blogRequestBody(t, blogRequest{Title: "Hello world", Visibility: entity.VisibilityPublic})
		req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), uid)
		rec := httptest.NewRecorder()
		s.CreateBlog(rec, req)

		if rec.Result().StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
		}
		slugs[uid] = decodeBlog(t, rec).Slug
	}

	if slugs["first"] != "hello-world" {
		t.Errorf("first posted at %q, want %q", slugs["first"], "hello-world")
	}
	if !strings.HasPrefix(slugs["second"], "hello-world-") {
		t.Errorf("second posted at %q, want another author's title to be taken already", slugs["second"])
	}
	if len(repo.blogs) != 2 {
		t.Errorf("stored %d posts, want neither to have displaced the other", len(repo.blogs))
	}
}

// A title nobody can slug is still a post that needs an address, and two of them still need
// telling apart - which is the same fallback a repeated title takes.
func TestCreateBlog_UntitledPostsAreStillAddressable(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newBlogService(repo, author("caller", "sly-dancing-monkey"))

	slugs := make([]string, 0, 2)
	for range 2 {
		body := blogRequestBody(t, blogRequest{Visibility: entity.VisibilityPublic})
		req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
		rec := httptest.NewRecorder()
		s.CreateBlog(rec, req)

		if rec.Result().StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
		}
		slugs = append(slugs, decodeBlog(t, rec).Slug)
	}

	if slugs[0] != "untitled" {
		t.Errorf("first slug = %q, want %q", slugs[0], "untitled")
	}
	if !strings.HasPrefix(slugs[1], "untitled-") || slugs[1] == slugs[0] {
		t.Errorf("second slug = %q, want a suffixed %q", slugs[1], "untitled")
	}
}

// A second post under a title cannot have the plain slug, so it takes a suffixed one rather than
// failing or overwriting the post already there.
func TestCreateBlog_SuffixesATitleAlreadyUsed(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newBlogService(repo, author("caller", "sly-dancing-monkey"))

	create := func() blogResponse {
		t.Helper()

		body := blogRequestBody(t, blogRequest{Title: "Hello world", Content: "Body", Visibility: entity.VisibilityPublic})
		req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
		rec := httptest.NewRecorder()
		s.CreateBlog(rec, req)

		if rec.Result().StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
		}
		return decodeBlog(t, rec)
	}

	first, second := create(), create()

	if first.Slug != "hello-world" {
		t.Errorf("first slug = %q, want %q", first.Slug, "hello-world")
	}
	if !strings.HasPrefix(second.Slug, "hello-world-") {
		t.Errorf("second slug = %q, want it to keep the title and add a suffix", second.Slug)
	}
	if suffix := strings.TrimPrefix(second.Slug, "hello-world-"); len(suffix) != 5 {
		t.Errorf("suffix of %q is %d characters, want 5", second.Slug, len(suffix))
	}
	// The first post must still be where it was: a colliding post takes a new slug, it does not
	// displace the one that got there first.
	if stored, _ := repo.stored("hello-world"); stored.CreatedAt != first.CreatedAt {
		t.Errorf("post at %q = %+v, want the first post left untouched", "hello-world", stored)
	}
	if len(repo.blogs) != 2 {
		t.Errorf("stored %d posts, want both to survive", len(repo.blogs))
	}
}

// A title that slugs to a name the frontend routes elsewhere ("New" onto /post/new) must not be
// posted there: the route would win over the slug beside it and the post would be unreachable.
func TestCreateBlog_AvoidsSlugsTheFrontendReserves(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newBlogService(repo, author("caller", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "New", Visibility: entity.VisibilityPublic})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	got := decodeBlog(t, rec)
	if got.Slug == "new" {
		t.Fatalf("Slug = %q, want a slug /post/new does not already claim", got.Slug)
	}
	if !strings.HasPrefix(got.Slug, "new-") {
		t.Errorf("Slug = %q, want the title kept and a suffix added", got.Slug)
	}
}

// Running out of draws is a server failure, not a bad request: the client never chose a slug, so
// there is nothing for it to correct.
func TestCreateBlog_FailsWhenNoSlugIsFree(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.beforeCreate = func(entity.Blog) error { return repository.ErrSlugTaken }
	s := newBlogService(repo, author("caller", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Hello world", Visibility: entity.VisibilityPublic})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusInternalServerError)
	}
	decodeAPIError(t, rec)
}

// Posting through a client always follows a profile being created, but nothing stops a caller
// reaching this endpoint first - and a post by an author with no username is attributed to nobody,
// so one is assigned rather than leaving the post unattributed.
func TestCreateBlog_NamesAnAuthorWhoHasNoProfile(t *testing.T) {
	blogs := newFakeBlogRepository()
	users := newFakeUserRepository()
	s := newTestService(blogs, users)

	body := blogRequestBody(t, blogRequest{Title: "Hello world", Visibility: entity.VisibilityPublic})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	got := decodeBlog(t, rec)
	if got.AuthorUsername == "" {
		t.Fatal("AuthorUsername is empty, want the post to have been given an author")
	}
	profile, ok := users.users["caller"]
	if !ok {
		t.Fatal("no profile was created for the author")
	}
	if profile.Username != got.AuthorUsername {
		t.Errorf("profile holds %q but the post carries %q", profile.Username, got.AuthorUsername)
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

// `tag` narrows a page to one topic, matching the normalized form a post stores rather than the
// spelling a link happened to carry.
func TestListBlogs_TagNarrowsToOneTopic(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "about-go", OwnerID: "author", Visibility: entity.VisibilityPublic, Tags: []string{"go", "web-dev"}},
		entity.Blog{Slug: "about-rust", OwnerID: "author", Visibility: entity.VisibilityPublic, Tags: []string{"rust"}},
		entity.Blog{Slug: "untagged", OwnerID: "author", Visibility: entity.VisibilityPublic},
	)
	s := newTestService(repo, nil)

	for _, query := range []string{"tag=web-dev", "tag=Web+Dev"} {
		t.Run(query, func(t *testing.T) {
			req := withUID(httptest.NewRequest(http.MethodGet, "/blogs?"+query, nil), "reader")
			rec := httptest.NewRecorder()
			s.ListBlogs(rec, req)

			if rec.Result().StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
			}
			var got blogListResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(got.Posts) != 1 || got.Posts[0].Slug != "about-go" {
				t.Errorf("posts = %v, want [about-go]", got.Posts)
			}
		})
	}
}

// `q` narrows a page to the posts holding a term in their title or body, ignoring case.
func TestListBlogs_QuerySearchesTitleAndContent(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "by-title", OwnerID: "author", Visibility: entity.VisibilityPublic, Title: "Firestore notes"},
		entity.Blog{Slug: "by-content", OwnerID: "author", Visibility: entity.VisibilityPublic, Content: "a post about firestore"},
		entity.Blog{Slug: "unrelated", OwnerID: "author", Visibility: entity.VisibilityPublic, Title: "Something else"},
	)
	s := newTestService(repo, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs?q=FIRESTORE", nil), "reader")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	slugs := make([]string, 0, len(got.Posts))
	for _, blog := range got.Posts {
		slugs = append(slugs, blog.Slug)
	}
	slices.Sort(slugs)
	if want := []string{"by-content", "by-title"}; !slices.Equal(slugs, want) {
		t.Errorf("slugs = %v, want %v", slugs, want)
	}
}

// Searching narrows what a reader could already have found by scrolling - it never widens it, so
// a post they may not read stays invisible however exactly they name it.
func TestListBlogs_SearchNeverSurfacesUnreadablePosts(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "secret", OwnerID: "someone", Visibility: entity.VisibilityPrivate, Title: "Firestore secrets", Tags: []string{"go"}},
	)
	s := newTestService(repo, nil)

	for _, query := range []string{"q=firestore", "tag=go"} {
		t.Run(query, func(t *testing.T) {
			req := withUID(httptest.NewRequest(http.MethodGet, "/blogs?"+query, nil), "reader")
			rec := httptest.NewRecorder()
			s.ListBlogs(rec, req)

			var got blogListResponse
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(got.Posts) != 0 {
				t.Errorf("posts = %v, want none", got.Posts)
			}
		})
	}
}

// The filters compose rather than exclude each other: "that author's posts about Go" is one page.
func TestListBlogs_FiltersCompose(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "wanted", OwnerID: "author", Visibility: entity.VisibilityPublic, Title: "Generics", Tags: []string{"go"}},
		entity.Blog{Slug: "wrong-author", OwnerID: "someone", Visibility: entity.VisibilityPublic, Title: "Generics", Tags: []string{"go"}},
		entity.Blog{Slug: "wrong-tag", OwnerID: "author", Visibility: entity.VisibilityPublic, Title: "Generics", Tags: []string{"rust"}},
		entity.Blog{Slug: "wrong-term", OwnerID: "author", Visibility: entity.VisibilityPublic, Title: "Channels", Tags: []string{"go"}},
	)
	s := newTestService(repo, nil)

	req := withUID(httptest.NewRequest(http.MethodGet, "/blogs?ownerId=author&tag=go&q=generics", nil), "reader")
	rec := httptest.NewRecorder()
	s.ListBlogs(rec, req)

	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Posts) != 1 || got.Posts[0].Slug != "wanted" {
		t.Errorf("posts = %v, want [wanted]", got.Posts)
	}
}

// Tags are normalized on the way in, so the form a post is stored - and so filtered and linked -
// under is decided by the server rather than by however the author typed them.
func TestCreateBlog_NormalizesTags(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newBlogService(repo, author("caller", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{
		Title:      "Hello, world!",
		Visibility: entity.VisibilityPublic,
		Tags:       []string{"Go", " Web Dev ", "go", "!!!"},
	})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	got := decodeBlog(t, rec)
	if want := []string{"go", "web-dev"}; !slices.Equal(got.Tags, want) {
		t.Errorf("tags = %v, want %v", got.Tags, want)
	}
	stored, ok := repo.stored(got.Slug)
	if !ok {
		t.Fatalf("no post stored at %q", got.Slug)
	}
	if !slices.Equal(stored.Tags, got.Tags) {
		t.Errorf("stored tags = %v, want %v", stored.Tags, got.Tags)
	}
}

func TestCreateBlog_RejectsTooManyTags(t *testing.T) {
	s := newTestService(nil, nil)

	tags := make([]string, entity.MaxTags+1)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag%d", i)
	}
	body := blogRequestBody(t, blogRequest{Title: "New post", Visibility: entity.VisibilityPublic, Tags: tags})
	req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), "caller")
	rec := httptest.NewRecorder()
	s.CreateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if got := decodeAPIError(t, rec).Error; !strings.Contains(got, "tags") {
		t.Errorf("error = %q, want it to name the tags field", got)
	}
}

// Retagging is a plain update, and leaves everything the request does not carry alone.
func TestUpdateBlog_ReplacesTags(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{
		Slug: "hello-world", OwnerID: "owner", Title: "Hello", Visibility: entity.VisibilityPublic,
		Tags: []string{"go", "web-dev"},
	})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Hello", Visibility: entity.VisibilityPublic, Tags: []string{"Rust"}})
	req := withUID(blogPathRequest(http.MethodPut, "hello-world", body), "owner")
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored, ok := repo.stored("hello-world")
	if !ok {
		t.Fatal("post is gone")
	}
	if want := []string{"rust"}; !slices.Equal(stored.Tags, want) {
		t.Errorf("tags = %v, want %v", stored.Tags, want)
	}

	// An update carrying no tags at all clears them, since a blog request is a full replace.
	body = blogRequestBody(t, blogRequest{Title: "Hello", Visibility: entity.VisibilityPublic})
	req = withUID(blogPathRequest(http.MethodPut, "hello-world", body), "owner")
	rec = httptest.NewRecorder()
	s.UpdateBlog(rec, req)

	if stored, _ = repo.stored("hello-world"); stored.Tags != nil {
		t.Errorf("tags = %v, want nil", stored.Tags)
	}
}

func TestUpdateBlog_ForbiddenForNonOwner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: entity.VisibilityPublic})
	req := blogPathRequest(http.MethodPut, "hello-world", body)
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "not-the-owner"))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
}

// A caller who cannot even read a private post gets the same 404 a missing one gives when they try
// to edit it too - not the 403 a non-owner gets on a post they can see, which would confirm the
// private post exists at that address.
func TestUpdateBlog_NotFoundForUnreadablePrivatePost(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"another"}})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: entity.VisibilityPublic})
	req := blogPathRequest(http.MethodPut, "hello-world", body)
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "not-the-owner"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

func TestUpdateBlog_NotFound(t *testing.T) {
	s := newBlogService(newFakeBlogRepository(), author("caller", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: entity.VisibilityPublic})
	req := blogPathRequest(http.MethodPut, "missing", body)
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
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic, CreatedAt: created})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := []byte(`{"title":"Edited","visibility":"public","ownerId":"someone-else","createdAt":"1999-01-01T00:00:00Z"}`)
	req := blogPathRequest(http.MethodPut, "hello-world", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored, ok := repo.stored("hello-world")
	if !ok {
		t.Fatalf("stored under %v, want the post left where it was", slices.Collect(maps.Keys(repo.blogs)))
	}
	if stored.OwnerID != "owner" {
		t.Errorf("OwnerID = %q, want %q (must ignore client-supplied ownerId)", stored.OwnerID, "owner")
	}
	if !stored.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v (must be carried over, not taken from the body)", stored.CreatedAt, created)
	}
}

func TestUpdateBlog_Owner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic, Title: "Original"})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: entity.VisibilityPrivate})
	req := blogPathRequest(http.MethodPut, "hello-world", body)
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored, _ := repo.stored("hello-world")
	if stored.Title != "Edited" {
		t.Errorf("Title = %q, want %q", stored.Title, "Edited")
	}
	if stored.Visibility != entity.VisibilityPrivate {
		t.Errorf("Visibility = %q, want %q", stored.Visibility, entity.VisibilityPrivate)
	}
}

// A slug is assigned once, from the title the post was created with. Re-deriving it on every edit
// would break every link to the post and free the old one for another post to take, so a retitled
// post keeps the address it has always had.
func TestUpdateBlog_RetitlingKeepsTheSlug(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic, Title: "Hello world"})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Something else entirely", Visibility: entity.VisibilityPublic})
	req := blogPathRequest(http.MethodPut, "hello-world", body)
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored, ok := repo.stored("hello-world")
	if !ok {
		t.Fatalf("stored under %v, want the post left at its original slug", slices.Collect(maps.Keys(repo.blogs)))
	}
	if stored.Title != "Something else entirely" {
		t.Errorf("Title = %q, want the edit applied", stored.Title)
	}
	if len(repo.blogs) != 1 {
		t.Errorf("stored %d posts, want the edit not to have created a second", len(repo.blogs))
	}
}

// An invalid body must not partially apply: the stored blog stays exactly as it was.
func TestUpdateBlog_InvalidBodyLeavesBlogUntouched(t *testing.T) {
	original := entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic, Title: "Original"}
	repo := newFakeBlogRepository()
	repo.seed(original)
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: "everyone"})
	req := blogPathRequest(http.MethodPut, "hello-world", body)
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	stored, _ := repo.stored("hello-world")
	if !reflect.DeepEqual(stored, original) {
		t.Errorf("stored = %+v, want it unchanged at %+v", stored, original)
	}
}

func TestDeleteBlog_ForbiddenForNonOwner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodDelete, "hello-world", nil)
	rec := httptest.NewRecorder()
	s.DeleteBlog(rec, withUID(req, "not-the-owner"))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if _, ok := repo.stored("hello-world"); !ok {
		t.Error("blog was deleted despite forbidden caller")
	}
}

// Deleting a post must never remove its Firestore document - only stamp DeletedAt on it - and the
// post must stop being reachable through the ordinary read path even though the document remains.
func TestDeleteBlog_Owner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodDelete, "hello-world", nil)
	rec := httptest.NewRecorder()
	s.DeleteBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	stored, ok := repo.stored("hello-world")
	if !ok {
		t.Fatal("blog document was removed by delete, want it kept and soft-deleted")
	}
	if stored.DeletedAt == nil {
		t.Error("DeletedAt is nil after delete, want it stamped")
	}
	if _, err := repo.Get(context.Background(), "hello-world"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get after delete = %v, want ErrNotFound", err)
	}
}

// A soft-deleted post reads as gone everywhere a client can see, even though its document is kept.
func TestDeleteBlog_PostIsNoLongerReadableOrListed(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodDelete, "hello-world", nil)
	rec := httptest.NewRecorder()
	s.DeleteBlog(rec, withUID(req, "owner"))
	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}

	getReq := blogPathRequest(http.MethodGet, "hello-world", nil)
	getRec := httptest.NewRecorder()
	s.GetBlog(getRec, withUID(getReq, "owner"))
	if getRec.Result().StatusCode != http.StatusNotFound {
		t.Errorf("GetBlog after delete status = %d, want %d", getRec.Result().StatusCode, http.StatusNotFound)
	}

	listRec := httptest.NewRecorder()
	s.ListBlogs(listRec, withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "owner"))
	var got blogListResponse
	if err := json.NewDecoder(listRec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Posts) != 0 {
		t.Errorf("ListBlogs after delete = %v, want the deleted post excluded", got.Posts)
	}
}

// A post carries its author's username, because a client cannot resolve one: profiles are
// addressable only by username, a post records its owner by uid, and the address no longer names
// the author at all.
func TestGetBlog_CarriesTheAuthor(t *testing.T) {
	blogs := newFakeBlogRepository()
	blogs.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(blogs, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodGet, "hello-world", nil)
	rec := httptest.NewRecorder()
	s.GetBlog(rec, withUID(req, "reader"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if got := decodeBlog(t, rec); got.AuthorUsername != "sly-dancing-monkey" {
		t.Errorf("AuthorUsername = %q, want %q", got.AuthorUsername, "sly-dancing-monkey")
	}
}

// Posts predate the rule that an author is named before posting, so the feed still has to render
// one whose owner holds no profile: an empty author, not a failed request.
func TestListBlogs_EmptyAuthorWhenTheOwnerHasNoProfile(t *testing.T) {
	blogs := newFakeBlogRepository()
	blogs.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newTestService(blogs, nil)

	rec := httptest.NewRecorder()
	s.ListBlogs(rec, withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "reader"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Posts) != 1 {
		t.Fatalf("posts = %d, want 1", len(got.Posts))
	}
	if got.Posts[0].AuthorUsername != "" {
		t.Errorf("AuthorUsername = %q, want it empty", got.Posts[0].AuthorUsername)
	}
}

// Authors are resolved per distinct owner, not per post, so one author's feed costs one lookup.
func TestListBlogs_ResolvesEachAuthorOnce(t *testing.T) {
	blogs := newFakeBlogRepository()
	for _, slug := range []string{"b1", "b2", "b3"} {
		blogs.seed(entity.Blog{Slug: slug, OwnerID: "owner", Visibility: entity.VisibilityPublic})
	}
	users := newFakeUserRepository()
	users.seed(entity.User{ID: "owner", Username: "sly-dancing-monkey"})
	s := newTestService(blogs, users)

	rec := httptest.NewRecorder()
	s.ListBlogs(rec, withUID(httptest.NewRequest(http.MethodGet, "/blogs", nil), "reader"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got blogListResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Posts) != 3 {
		t.Fatalf("posts = %d, want 3", len(got.Posts))
	}
	for _, post := range got.Posts {
		if post.AuthorUsername != "sly-dancing-monkey" {
			t.Fatalf("post %s carries author %q, want the owner's username", post.Slug, post.AuthorUsername)
		}
	}
	if users.gets != 1 {
		t.Errorf("user lookups = %d, want 1 for three posts by one author", users.gets)
	}
}
