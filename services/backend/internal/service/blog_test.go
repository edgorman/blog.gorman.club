package service

import (
	"bytes"
	"encoding/json"
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

// blogPathRequest addresses a post the way its route does: by its author's username and its slug.
// The target is escaped but the path values are not, since that is what a real request produces -
// ServeMux decodes each segment before a handler reads it.
func blogPathRequest(method, username, slug string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, "/blogs/"+url.PathEscape(username)+"/"+url.PathEscape(slug), body)
	req.SetPathValue("username", username)
	req.SetPathValue("slug", slug)
	return req
}

// newBlogService wires a service over posts and the profiles they are addressed through. Seeding
// the authors is not optional scaffolding: a request names a post by its author's username, and
// only a profile maps that back to the uid the post records as its owner.
func newBlogService(blogs repository.BlogRepository, authors ...entity.User) *Service {
	users := newFakeUserRepository()
	for _, author := range authors {
		users.seed(author)
	}
	return newTestService(blogs, users)
}

// author is the profile behind a post's owner uid, named as the URL to it would be.
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
	var got []entity.Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	slugs := make([]string, 0, len(got))
	for _, blog := range got {
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

	var got []entity.Blog
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 || got[0].Slug != "newer" || got[1].Slug != "older" {
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
		{"public post", entity.Blog{Slug: "hello-world", OwnerID: "someone", Visibility: entity.VisibilityPublic}},
		{"own private post", entity.Blog{Slug: "hello-world", OwnerID: "caller", Visibility: entity.VisibilityPrivate}},
		{"whitelisted private post", entity.Blog{Slug: "hello-world", OwnerID: "someone", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"caller"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeBlogRepository()
			repo.seed(tt.blog)
			s := newBlogService(repo, author(tt.blog.OwnerID, "sly-dancing-monkey"))

			req := blogPathRequest(http.MethodGet, "sly-dancing-monkey", "hello-world", nil)
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

// Slugs belong to an author, so the same one under two usernames is two different posts - and
// asking under the wrong author must not reach the other's.
func TestGetBlog_ResolvesTheSlugUnderItsAuthor(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(
		entity.Blog{Slug: "hello-world", OwnerID: "first", Title: "First author", Visibility: entity.VisibilityPublic},
		entity.Blog{Slug: "hello-world", OwnerID: "second", Title: "Second author", Visibility: entity.VisibilityPublic},
	)
	s := newBlogService(repo, author("first", "sly-dancing-monkey"), author("second", "bold-leaping-otter"))

	for username, want := range map[string]string{
		"sly-dancing-monkey": "First author",
		"bold-leaping-otter": "Second author",
	} {
		req := blogPathRequest(http.MethodGet, username, "hello-world", nil)
		rec := httptest.NewRecorder()
		s.GetBlog(rec, withUID(req, "reader"))

		if rec.Result().StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
		}
		if got := decodeBlog(t, rec); got.Title != want {
			t.Errorf("/blogs/%s/hello-world = %q, want %q", username, got.Title, want)
		}
	}
}

// A caller who cannot read a private post is shown the same 404 a missing post gets, not a 403:
// the address is a function of the author's username and the post's own title, so a 403 would let
// a stranger confirm the post exists just by guessing at it.
func TestGetBlog_NotFoundForUnreadablePrivatePost(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"another"}})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodGet, "sly-dancing-monkey", "hello-world", nil)
	rec := httptest.NewRecorder()
	s.GetBlog(rec, withUID(req, "caller"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

// A username nobody holds and a post the holder does not have are the same 404: either way the
// post asked for does not exist, and which half missed is not the caller's business.
func TestGetBlog_NotFound(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})

	for _, tt := range []struct{ name, username, slug string }{
		{"unknown author", "nobody-at-all", "hello-world"},
		{"author holds no such post", "sly-dancing-monkey", "missing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

			req := blogPathRequest(http.MethodGet, tt.username, tt.slug, nil)
			rec := httptest.NewRecorder()
			s.GetBlog(rec, withUID(req, "caller"))

			if rec.Result().StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
			}
			decodeAPIError(t, rec)
		})
	}
}

// A malformed address gets the same 404 a well-formed but absent one does, rather than a 400
// naming the rule it broke: the path is a URL a reader followed, and the shape of a rejected slug
// would tell a prober as much about what exists as an outright miss would.
func TestGetBlog_NotFoundForMalformedAddresses(t *testing.T) {
	for _, tt := range []struct{ name, username, slug string }{
		{"bad username", "not a username", "hello-world"},
		{"bad slug", "sly-dancing-monkey", "Hello World"},
		{"empty slug", "sly-dancing-monkey", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newBlogService(newFakeBlogRepository(), author("owner", "sly-dancing-monkey"))

			req := blogPathRequest(http.MethodGet, tt.username, tt.slug, nil)
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

// A post is addressed by its author's username, so the response carries one: without it a client
// holds a slug it cannot build a URL from.
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
		t.Errorf("AuthorUsername = %q, want the address to be complete", got.AuthorUsername)
	}
	if _, stored := repo.stored("caller", "hello-world"); !stored {
		t.Errorf("stored under %v, want the post written at its author and slug", slices.Collect(maps.Keys(repo.blogs)))
	}
}

// Slugs are scoped to their author, so a title one person has used says nothing about whether
// another may use it - which is the whole difference from a globally unique id.
func TestCreateBlog_SlugsAreScopedToTheAuthor(t *testing.T) {
	repo := newFakeBlogRepository()
	s := newBlogService(repo, author("first", "sly-dancing-monkey"), author("second", "bold-leaping-otter"))

	for _, uid := range []string{"first", "second"} {
		body := blogRequestBody(t, blogRequest{Title: "Hello world", Visibility: entity.VisibilityPublic})
		req := withUID(httptest.NewRequest(http.MethodPost, "/blogs", body), uid)
		rec := httptest.NewRecorder()
		s.CreateBlog(rec, req)

		if rec.Result().StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
		}
		if got := decodeBlog(t, rec); got.Slug != "hello-world" {
			t.Errorf("%s posted at %q, want %q - one author's slug is not another's", uid, got.Slug, "hello-world")
		}
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

// An author's second post under one title cannot have the plain slug, so it takes a suffixed one
// rather than failing or overwriting the post already there.
func TestCreateBlog_SuffixesATitleTheAuthorHasUsed(t *testing.T) {
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
	if stored, _ := repo.stored("caller", "hello-world"); stored.CreatedAt != first.CreatedAt {
		t.Errorf("post at %q = %+v, want the first post left untouched", "hello-world", stored)
	}
	if len(repo.blogs) != 2 {
		t.Errorf("stored %d posts, want both to survive", len(repo.blogs))
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
// reaching this endpoint first - and a post by an author with no username has no address at all,
// so one is assigned rather than leaving the post unreachable.
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
		t.Fatal("AuthorUsername is empty, want the post to have been given an address")
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

func TestUpdateBlog_ForbiddenForNonOwner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Edited", Visibility: entity.VisibilityPublic})
	req := blogPathRequest(http.MethodPut, "sly-dancing-monkey", "hello-world", body)
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
	req := blogPathRequest(http.MethodPut, "sly-dancing-monkey", "hello-world", body)
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
	req := blogPathRequest(http.MethodPut, "sly-dancing-monkey", "missing", body)
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
	req := blogPathRequest(http.MethodPut, "sly-dancing-monkey", "hello-world", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored, ok := repo.stored("owner", "hello-world")
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
	req := blogPathRequest(http.MethodPut, "sly-dancing-monkey", "hello-world", body)
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored, _ := repo.stored("owner", "hello-world")
	if stored.Title != "Edited" {
		t.Errorf("Title = %q, want %q", stored.Title, "Edited")
	}
	if stored.Visibility != entity.VisibilityPrivate {
		t.Errorf("Visibility = %q, want %q", stored.Visibility, entity.VisibilityPrivate)
	}
}

// A slug is assigned once, from the title the post was created with. Re-deriving it on every edit
// would break every link to the post and free the old one for another of the author's posts, so a
// retitled post keeps the address it has always had.
func TestUpdateBlog_RetitlingKeepsTheSlug(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic, Title: "Hello world"})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	body := blogRequestBody(t, blogRequest{Title: "Something else entirely", Visibility: entity.VisibilityPublic})
	req := blogPathRequest(http.MethodPut, "sly-dancing-monkey", "hello-world", body)
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	stored, ok := repo.stored("owner", "hello-world")
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
	req := blogPathRequest(http.MethodPut, "sly-dancing-monkey", "hello-world", body)
	rec := httptest.NewRecorder()
	s.UpdateBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	stored, _ := repo.stored("owner", "hello-world")
	if !reflect.DeepEqual(stored, original) {
		t.Errorf("stored = %+v, want it unchanged at %+v", stored, original)
	}
}

func TestDeleteBlog_ForbiddenForNonOwner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodDelete, "sly-dancing-monkey", "hello-world", nil)
	rec := httptest.NewRecorder()
	s.DeleteBlog(rec, withUID(req, "not-the-owner"))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if _, ok := repo.stored("owner", "hello-world"); !ok {
		t.Error("blog was deleted despite forbidden caller")
	}
}

func TestDeleteBlog_Owner(t *testing.T) {
	repo := newFakeBlogRepository()
	repo.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(repo, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodDelete, "sly-dancing-monkey", "hello-world", nil)
	rec := httptest.NewRecorder()
	s.DeleteBlog(rec, withUID(req, "owner"))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if _, ok := repo.stored("owner", "hello-world"); ok {
		t.Error("blog still present after delete")
	}
}

// A post carries its author's username, because a client cannot resolve one: profiles are
// addressable only by username, and a post records its owner by uid.
func TestGetBlog_CarriesTheAuthor(t *testing.T) {
	blogs := newFakeBlogRepository()
	blogs.seed(entity.Blog{Slug: "hello-world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
	s := newBlogService(blogs, author("owner", "sly-dancing-monkey"))

	req := blogPathRequest(http.MethodGet, "sly-dancing-monkey", "hello-world", nil)
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
	var got []blogResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("posts = %d, want 1", len(got))
	}
	if got[0].AuthorUsername != "" {
		t.Errorf("AuthorUsername = %q, want it empty", got[0].AuthorUsername)
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
	var got []blogResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("posts = %d, want 3", len(got))
	}
	for _, post := range got {
		if post.AuthorUsername != "sly-dancing-monkey" {
			t.Fatalf("post %s carries author %q, want the owner's username", post.Slug, post.AuthorUsername)
		}
	}
	if users.gets != 1 {
		t.Errorf("user lookups = %d, want 1 for three posts by one author", users.gets)
	}
}
