package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// commentFixture is a post, its owner's profile, and the thread beneath it, wired the way a real
// request finds them: the post's owner holds a profile, since every author does (see ensureAuthor).
type commentFixture struct {
	service  *Service
	blogs    *fakeBlogRepository
	users    *fakeUserRepository
	comments *fakeCommentRepository
}

const (
	commentSlug   = "hello-world"
	commentAuthor = "author"
	commentReader = "reader"
)

// newCommentFixture seeds a public post. Tests that need a private one edit it through blogs.
func newCommentFixture(t *testing.T) *commentFixture {
	t.Helper()

	blogs := newFakeBlogRepository()
	blogs.seed(entity.Blog{
		Slug:       commentSlug,
		OwnerID:    commentAuthor,
		Title:      "Hello",
		Content:    "the cat sat",
		Visibility: entity.VisibilityPublic,
	})

	users := newFakeUserRepository()
	users.seed(entity.User{ID: commentAuthor, Username: "calm-smiling-kestrel"})
	users.seed(entity.User{ID: commentReader, Username: "sly-dancing-monkey"})

	comments := newFakeCommentRepository()

	return &commentFixture{
		service:  newCommentService(blogs, users, comments),
		blogs:    blogs,
		users:    users,
		comments: comments,
	}
}

// post sends a comment as uid, or anonymously when uid is empty.
func (f *commentFixture) post(uid, body string) *httptest.ResponseRecorder {
	encoded, _ := json.Marshal(commentRequest{Body: body})
	req := httptest.NewRequest(http.MethodPost, "/blogs/"+commentSlug+"/comments", bytes.NewReader(encoded))
	req.SetPathValue("slug", commentSlug)
	if uid != "" {
		req = withUID(req, uid)
	}

	rec := httptest.NewRecorder()
	f.service.CreateComment(rec, req)
	return rec
}

func (f *commentFixture) list(uid string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/blogs/"+commentSlug+"/comments", nil)
	req.SetPathValue("slug", commentSlug)
	if uid != "" {
		req = withUID(req, uid)
	}

	rec := httptest.NewRecorder()
	f.service.ListComments(rec, req)
	return rec
}

func (f *commentFixture) delete(uid, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/blogs/"+commentSlug+"/comments/"+id, nil)
	req.SetPathValue("slug", commentSlug)
	req.SetPathValue("id", id)
	if uid != "" {
		req = withUID(req, uid)
	}

	rec := httptest.NewRecorder()
	f.service.DeleteComment(rec, req)
	return rec
}

func decodeComment(t *testing.T, rec *httptest.ResponseRecorder) commentResponse {
	t.Helper()

	var body commentResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode comment: %v", err)
	}
	return body
}

func decodeComments(t *testing.T, rec *httptest.ResponseRecorder) []commentResponse {
	t.Helper()

	var body []commentResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode comments: %v", err)
	}
	return body
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, want, rec.Body.String())
	}
}

func TestCreateComment(t *testing.T) {
	f := newCommentFixture(t)

	rec := f.post(commentReader, "  nicely put  ")
	assertStatus(t, rec, http.StatusCreated)

	created := decodeComment(t, rec)
	if created.Body != "nicely put" {
		t.Errorf("Body = %q, want it trimmed", created.Body)
	}
	if created.AuthorID != commentReader {
		t.Errorf("AuthorID = %q, want the caller %q", created.AuthorID, commentReader)
	}
	if created.BlogSlug != commentSlug {
		t.Errorf("BlogSlug = %q, want the post from the path", created.BlogSlug)
	}
	// The uid a comment records its author by is never public, so the username resolved here is
	// the only handle a client has for the profile behind it.
	if created.AuthorUsername != "sly-dancing-monkey" {
		t.Errorf("AuthorUsername = %q, want the commenter's username", created.AuthorUsername)
	}
	if created.ID == "" {
		t.Error("ID is empty, want the repository to have assigned one")
	}

	if stored := f.comments.threads[commentSlug]; len(stored) != 1 {
		t.Fatalf("stored %d comments, want 1", len(stored))
	}
}

// The author of the post is a reader of it like any other, and may reply beneath their own work.
func TestCreateComment_ByThePostsOwner(t *testing.T) {
	f := newCommentFixture(t)

	assertStatus(t, f.post(commentAuthor, "thanks all"), http.StatusCreated)
}

func TestCreateComment_Invalid(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"only whitespace", "   "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newCommentFixture(t)

			rec := f.post(commentReader, tt.body)
			assertStatus(t, rec, http.StatusBadRequest)
			decodeAPIError(t, rec)

			if len(f.comments.threads[commentSlug]) != 0 {
				t.Error("a rejected comment was stored")
			}
		})
	}
}

// A comment is shown by username, so a caller who never set up a profile is given one here for the
// same reason publishing gives an author one - otherwise their comment would be attributed to
// nobody.
func TestCreateComment_NamesACallerWithNoProfile(t *testing.T) {
	f := newCommentFixture(t)

	rec := f.post("newcomer", "first time here")
	assertStatus(t, rec, http.StatusCreated)

	if username := decodeComment(t, rec).AuthorUsername; username == "" {
		t.Error("AuthorUsername is empty, want a profile to have been assigned")
	}
}

// A profile is only worth creating for a comment that was actually accepted.
func TestCreateComment_LeavesNoProfileForARejectedComment(t *testing.T) {
	f := newCommentFixture(t)

	assertStatus(t, f.post("newcomer", "  "), http.StatusBadRequest)

	if _, ok := f.users.users["newcomer"]; ok {
		t.Error("a profile was created for a caller whose comment was refused")
	}
}

// A private post's thread is as private as the post: a caller who cannot read one is answered with
// the post's own 404, rather than a 403 or an empty thread that would admit it exists.
func TestComments_OnAPrivatePost(t *testing.T) {
	newPrivateFixture := func(t *testing.T) *commentFixture {
		t.Helper()

		f := newCommentFixture(t)
		post, _ := f.blogs.stored(commentSlug)
		post.Visibility = entity.VisibilityPrivate
		post.AllowedUserIDs = []string{commentReader}
		f.blogs.seed(post)
		return f
	}

	t.Run("a whitelisted reader may comment", func(t *testing.T) {
		f := newPrivateFixture(t)

		assertStatus(t, f.post(commentReader, "nicely put"), http.StatusCreated)
		assertStatus(t, f.list(commentReader), http.StatusOK)
	})

	t.Run("a stranger sees the post's own 404", func(t *testing.T) {
		f := newPrivateFixture(t)

		assertStatus(t, f.post("stranger", "nicely put"), http.StatusNotFound)
		assertStatus(t, f.list("stranger"), http.StatusNotFound)

		if len(f.comments.threads[commentSlug]) != 0 {
			t.Error("a comment was stored on a post the caller cannot read")
		}
	})
}

func TestListComments(t *testing.T) {
	f := newCommentFixture(t)
	assertStatus(t, f.post(commentReader, "first"), http.StatusCreated)
	assertStatus(t, f.post(commentAuthor, "second"), http.StatusCreated)

	// Signed out: a public post's thread is readable by anyone who can read the post.
	rec := f.list("")
	assertStatus(t, rec, http.StatusOK)

	comments := decodeComments(t, rec)
	if len(comments) != 2 {
		t.Fatalf("listed %d comments, want 2", len(comments))
	}
	if comments[0].Body != "first" || comments[1].Body != "second" {
		t.Errorf("bodies = %q, %q, want them oldest first", comments[0].Body, comments[1].Body)
	}
	if comments[0].AuthorUsername != "sly-dancing-monkey" || comments[1].AuthorUsername != "calm-smiling-kestrel" {
		t.Error("comments are not attributed to the profiles that wrote them")
	}
}

// A post nobody has commented on has an empty thread, not a missing one - and it is an empty JSON
// array rather than null, so a client renders it without a special case.
func TestListComments_Empty(t *testing.T) {
	f := newCommentFixture(t)

	rec := f.list("")
	assertStatus(t, rec, http.StatusOK)

	if body := rec.Body.String(); body != "[]\n" {
		t.Errorf("body = %q, want an empty JSON array", body)
	}
}

// Each distinct commenter is resolved once however many comments they left, so a thread dominated
// by one person costs one profile read rather than one per comment.
func TestListComments_ResolvesEachAuthorOnce(t *testing.T) {
	f := newCommentFixture(t)
	for range 3 {
		assertStatus(t, f.post(commentReader, "again"), http.StatusCreated)
	}

	f.users.gets = 0
	assertStatus(t, f.list(""), http.StatusOK)

	if f.users.gets != 1 {
		t.Errorf("profile lookups = %d, want 1 for three comments by one author", f.users.gets)
	}
}

func TestDeleteComment(t *testing.T) {
	for _, tt := range []struct {
		name string
		uid  string
		want int
	}{
		{"its author retracts it", commentReader, http.StatusNoContent},
		{"the post's owner moderates it", commentAuthor, http.StatusNoContent},
		{"another reader may not", "stranger", http.StatusForbidden},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newCommentFixture(t)
			created := decodeComment(t, f.post(commentReader, "nicely put"))

			rec := f.delete(tt.uid, created.ID)
			assertStatus(t, rec, tt.want)

			deleted := len(f.comments.threads[commentSlug]) == 0
			if deleted != (tt.want == http.StatusNoContent) {
				t.Errorf("comment deleted = %v, want %v", deleted, tt.want == http.StatusNoContent)
			}
		})
	}
}

func TestDeleteComment_Missing(t *testing.T) {
	for _, tt := range []struct {
		name string
		id   string
	}{
		{"no such comment", "cmt404"},
		// A malformed id is folded into the same "nothing here" a missing one gets, exactly as a
		// malformed slug is: the path is a link somebody followed, not a form they filled in.
		{"malformed", "not/an/id"},
		{"empty", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newCommentFixture(t)

			rec := f.delete(commentAuthor, tt.id)
			assertStatus(t, rec, http.StatusNotFound)
			decodeAPIError(t, rec)
		})
	}
}

// Comments are addressed beneath their post, so an id from one thread names nothing in another -
// which is what stops the owner of any post from deleting a comment on somebody else's.
func TestDeleteComment_FromAnotherPostsThread(t *testing.T) {
	f := newCommentFixture(t)
	f.blogs.seed(entity.Blog{
		Slug:       "other-post",
		OwnerID:    "somebody-else",
		Visibility: entity.VisibilityPublic,
	})
	created := decodeComment(t, f.post(commentReader, "nicely put"))

	req := httptest.NewRequest(http.MethodDelete, "/blogs/other-post/comments/"+created.ID, nil)
	req.SetPathValue("slug", "other-post")
	req.SetPathValue("id", created.ID)
	rec := httptest.NewRecorder()
	f.service.DeleteComment(rec, withUID(req, "somebody-else"))

	assertStatus(t, rec, http.StatusNotFound)
	if len(f.comments.threads[commentSlug]) != 1 {
		t.Error("a comment was deleted through another post's thread")
	}
}

func TestComments_OnAMissingPost(t *testing.T) {
	f := newCommentFixture(t)

	for _, tt := range []struct {
		name string
		run  func() *httptest.ResponseRecorder
	}{
		{"list", func() *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodGet, "/blogs/missing/comments", nil)
			req.SetPathValue("slug", "missing")
			rec := httptest.NewRecorder()
			f.service.ListComments(rec, req)
			return rec
		}},
		{"create", func() *httptest.ResponseRecorder {
			encoded, _ := json.Marshal(commentRequest{Body: "nicely put"})
			req := httptest.NewRequest(http.MethodPost, "/blogs/missing/comments", bytes.NewReader(encoded))
			req.SetPathValue("slug", "missing")
			rec := httptest.NewRecorder()
			f.service.CreateComment(rec, withUID(req, commentReader))
			return rec
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assertStatus(t, tt.run(), http.StatusNotFound)
		})
	}
}

// A write the datastore refused is a 500 rather than a partial success, and stores nothing.
func TestCreateComment_RepositoryFailure(t *testing.T) {
	f := newCommentFixture(t)
	f.comments.createErr = errors.New("firestore is down")

	rec := f.post(commentReader, "nicely put")
	assertStatus(t, rec, http.StatusInternalServerError)
	decodeAPIError(t, rec)
}
