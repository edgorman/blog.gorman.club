package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// reactionFixture is a post with a comment on it and the reactions to both, wired the way a real
// request finds them.
type reactionFixture struct {
	service   *Service
	blogs     *fakeBlogRepository
	comments  *fakeCommentRepository
	reactions *fakeReactionRepository
}

const reactionCommentID = "cmt1"

func newReactionFixture(t *testing.T) *reactionFixture {
	t.Helper()

	blogs := newFakeBlogRepository()
	blogs.seed(entity.Blog{
		Slug:       commentSlug,
		OwnerID:    commentAuthor,
		Title:      "Hello",
		Visibility: entity.VisibilityPublic,
	})

	users := newFakeUserRepository()
	users.seed(entity.User{ID: commentAuthor, Username: "calm-smiling-kestrel"})

	comments := newFakeCommentRepository()
	comments.seed(entity.Comment{
		ID:       reactionCommentID,
		BlogSlug: commentSlug,
		AuthorID: commentReader,
		Body:     "Nicely put.",
	})

	reactions := newFakeReactionRepository()

	return &reactionFixture{
		service:   newReactionService(blogs, users, comments, reactions),
		blogs:     blogs,
		comments:  comments,
		reactions: reactions,
	}
}

// react sends a PUT or DELETE for one emoji on the post, or on its comment when commentID is set.
func (f *reactionFixture) react(method, uid, commentID, emoji string) *httptest.ResponseRecorder {
	path := "/blogs/" + commentSlug + "/reactions/" + url.PathEscape(emoji)
	if commentID != "" {
		path = "/blogs/" + commentSlug + "/comments/" + commentID + "/reactions/" + url.PathEscape(emoji)
	}

	req := httptest.NewRequest(method, path, nil)
	req.SetPathValue("slug", commentSlug)
	req.SetPathValue("emoji", emoji)
	if commentID != "" {
		req.SetPathValue("id", commentID)
	}
	if uid != "" {
		req = withUID(req, uid)
	}

	rec := httptest.NewRecorder()
	if method == http.MethodDelete {
		f.service.DeleteReaction(rec, req)
	} else {
		f.service.PutReaction(rec, req)
	}
	return rec
}

func (f *reactionFixture) list(uid string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/blogs/"+commentSlug+"/reactions", nil)
	req.SetPathValue("slug", commentSlug)
	if uid != "" {
		req = withUID(req, uid)
	}

	rec := httptest.NewRecorder()
	f.service.GetReactions(rec, req)
	return rec
}

func decodeCounts(t *testing.T, rec *httptest.ResponseRecorder) []reactionCount {
	t.Helper()

	var counts []reactionCount
	if err := json.NewDecoder(rec.Body).Decode(&counts); err != nil {
		t.Fatalf("decode reaction counts: %v", err)
	}
	return counts
}

func decodeReactions(t *testing.T, rec *httptest.ResponseRecorder) reactionsResponse {
	t.Helper()

	var body reactionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode reactions: %v", err)
	}
	return body
}

func TestPutReaction_OnAPost(t *testing.T) {
	f := newReactionFixture(t)

	rec := f.react(http.MethodPut, commentReader, "", "👍")
	assertStatus(t, rec, http.StatusOK)

	counts := decodeCounts(t, rec)
	if len(counts) != 1 || counts[0].Emoji != "👍" || counts[0].Count != 1 || !counts[0].Reacted {
		t.Fatalf("counts = %+v, want one 👍 the caller is in", counts)
	}
}

func TestPutReaction_OnAComment(t *testing.T) {
	f := newReactionFixture(t)

	assertStatus(t, f.react(http.MethodPut, commentReader, reactionCommentID, "🎉"), http.StatusOK)

	stored := f.reactions.reactions[entity.Reaction{
		Target: entity.CommentReaction(commentSlug, reactionCommentID),
		UID:    commentReader,
	}.Key()]
	if len(stored.Emojis) != 1 || stored.Emojis[0] != "🎉" {
		t.Errorf("stored = %+v, want 🎉 on the comment", stored)
	}
}

// The bar is a shared count, so a write answers with what everybody's clicks have made of it -
// which an optimistic +1 on the client could not know.
func TestPutReaction_CountsEveryReader(t *testing.T) {
	f := newReactionFixture(t)
	assertStatus(t, f.react(http.MethodPut, "reader-one", "", "👍"), http.StatusOK)

	rec := f.react(http.MethodPut, "reader-two", "", "👍")
	assertStatus(t, rec, http.StatusOK)

	counts := decodeCounts(t, rec)
	if len(counts) != 1 || counts[0].Count != 2 {
		t.Fatalf("counts = %+v, want 👍 counted twice", counts)
	}
}

// PUT says what should be true rather than asking for a flip, so a retried click or a stale page
// lands where it was aiming instead of undoing itself.
func TestPutReaction_IsIdempotent(t *testing.T) {
	f := newReactionFixture(t)

	assertStatus(t, f.react(http.MethodPut, commentReader, "", "👍"), http.StatusOK)
	rec := f.react(http.MethodPut, commentReader, "", "👍")
	assertStatus(t, rec, http.StatusOK)

	counts := decodeCounts(t, rec)
	if len(counts) != 1 || counts[0].Count != 1 {
		t.Fatalf("counts = %+v, want the same reader counted once", counts)
	}
}

func TestDeleteReaction(t *testing.T) {
	f := newReactionFixture(t)
	assertStatus(t, f.react(http.MethodPut, commentReader, "", "👍"), http.StatusOK)

	rec := f.react(http.MethodDelete, commentReader, "", "👍")
	assertStatus(t, rec, http.StatusOK)

	if counts := decodeCounts(t, rec); len(counts) != 0 {
		t.Fatalf("counts = %+v, want none left", counts)
	}
	// A reader with nothing left is erased rather than kept as an empty row.
	if len(f.reactions.reactions) != 0 {
		t.Errorf("stored = %+v, want the empty record removed", f.reactions.reactions)
	}
}

func TestDeleteReaction_IsIdempotent(t *testing.T) {
	f := newReactionFixture(t)

	assertStatus(t, f.react(http.MethodDelete, commentReader, "", "👍"), http.StatusOK)
}

// One reader's click never removes another's, even for the same emoji.
func TestDeleteReaction_LeavesOtherReaders(t *testing.T) {
	f := newReactionFixture(t)
	assertStatus(t, f.react(http.MethodPut, "reader-one", "", "👍"), http.StatusOK)
	assertStatus(t, f.react(http.MethodPut, "reader-two", "", "👍"), http.StatusOK)

	rec := f.react(http.MethodDelete, "reader-one", "", "👍")
	assertStatus(t, rec, http.StatusOK)

	counts := decodeCounts(t, rec)
	if len(counts) != 1 || counts[0].Count != 1 {
		t.Fatalf("counts = %+v, want the other reader's 👍 to remain", counts)
	}
	if counts[0].Reacted {
		t.Error("Reacted = true, want the caller no longer in it")
	}
}

// Any emoji works, which is the point: the picker is a convenience, not the rule.
func TestPutReaction_AcceptsAnyEmoji(t *testing.T) {
	for _, emoji := range []string{"👍", "🎉", "🫠", "👨‍👩‍👧‍👦", "🇬🇧", "👍🏽", "⭐"} {
		t.Run(emoji, func(t *testing.T) {
			f := newReactionFixture(t)

			assertStatus(t, f.react(http.MethodPut, commentReader, "", emoji), http.StatusOK)
		})
	}
}

func TestPutReaction_RejectsWhatIsNotAnEmoji(t *testing.T) {
	for _, tt := range []struct{ name, emoji string }{
		{"a word", "nice"},
		{"an emoji in a sentence", "nice 👍"},
		{"two emoji", "👍👎"},
		{"empty", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newReactionFixture(t)

			rec := f.react(http.MethodPut, commentReader, "", tt.emoji)
			assertStatus(t, rec, http.StatusBadRequest)
			decodeAPIError(t, rec)

			if len(f.reactions.reactions) != 0 {
				t.Error("a rejected reaction was stored")
			}
		})
	}
}

// A reader who has filled their own row is told so rather than served a 500.
func TestPutReaction_BoundedPerReader(t *testing.T) {
	f := newReactionFixture(t)
	emojis := []string{"👍", "👎", "😀", "😂", "😍", "🎉", "🔥", "💯", "🚀", "👀", "🙏", "✅", "⭐"}

	for _, emoji := range emojis[:entity.MaxReactionsPerTarget] {
		assertStatus(t, f.react(http.MethodPut, commentReader, "", emoji), http.StatusOK)
	}

	rec := f.react(http.MethodPut, commentReader, "", emojis[entity.MaxReactionsPerTarget])
	assertStatus(t, rec, http.StatusBadRequest)
	decodeAPIError(t, rec)
}

// A reaction to a comment nobody wrote would be counted by nothing and outlive every cleanup, so
// the target is checked to exist rather than taken from the path on trust.
func TestPutReaction_OnAMissingComment(t *testing.T) {
	f := newReactionFixture(t)

	rec := f.react(http.MethodPut, commentReader, "cmt404", "👍")
	assertStatus(t, rec, http.StatusNotFound)

	if len(f.reactions.reactions) != 0 {
		t.Error("a reaction was stored against a comment that does not exist")
	}
}

func TestGetReactions(t *testing.T) {
	f := newReactionFixture(t)
	assertStatus(t, f.react(http.MethodPut, commentReader, "", "👍"), http.StatusOK)
	assertStatus(t, f.react(http.MethodPut, commentAuthor, "", "👍"), http.StatusOK)
	assertStatus(t, f.react(http.MethodPut, commentAuthor, "", "🎉"), http.StatusOK)
	assertStatus(t, f.react(http.MethodPut, commentReader, reactionCommentID, "🔥"), http.StatusOK)

	// Signed out: a public post's reactions are readable by anyone who may read the post.
	rec := f.list("")
	assertStatus(t, rec, http.StatusOK)

	body := decodeReactions(t, rec)
	if len(body.Post) != 2 {
		t.Fatalf("post reactions = %+v, want two emoji", body.Post)
	}
	// Most chosen first, so the bar does not reorder itself as people click.
	if body.Post[0].Emoji != "👍" || body.Post[0].Count != 2 || body.Post[1].Count != 1 {
		t.Errorf("post reactions = %+v, want 👍 (2) before 🎉 (1)", body.Post)
	}
	if body.Post[0].Reacted {
		t.Error("Reacted = true for a signed-out reader, want false")
	}
	if counts := body.Comments[reactionCommentID]; len(counts) != 1 || counts[0].Emoji != "🔥" {
		t.Errorf("comment reactions = %+v, want one 🔥", counts)
	}
}

func TestGetReactions_ReportsWhetherTheCallerIsIn(t *testing.T) {
	f := newReactionFixture(t)
	assertStatus(t, f.react(http.MethodPut, commentReader, "", "👍"), http.StatusOK)

	mine := decodeReactions(t, f.list(commentReader))
	if !mine.Post[0].Reacted {
		t.Error("Reacted = false for the reader who reacted, want true")
	}

	theirs := decodeReactions(t, f.list("somebody-else"))
	if theirs.Post[0].Reacted {
		t.Error("Reacted = true for a reader who did not, want false")
	}
	if theirs.Post[0].Count != 1 {
		t.Errorf("Count = %d, want the other reader still counted", theirs.Post[0].Count)
	}
}

func TestGetReactions_Empty(t *testing.T) {
	f := newReactionFixture(t)

	rec := f.list("")
	assertStatus(t, rec, http.StatusOK)

	body := decodeReactions(t, rec)
	if len(body.Post) != 0 || len(body.Comments) != 0 {
		t.Errorf("body = %+v, want nothing on a post nobody has reacted to", body)
	}
}

// A private post's reactions are as private as the post: a caller who cannot see it gets the
// post's own 404 rather than a count that would admit it exists.
func TestReactions_OnAPrivatePost(t *testing.T) {
	newPrivateFixture := func(t *testing.T) *reactionFixture {
		t.Helper()

		f := newReactionFixture(t)
		post, _ := f.blogs.stored(commentSlug)
		post.Visibility = entity.VisibilityPrivate
		post.AllowedUserIDs = []string{commentReader}
		f.blogs.seed(post)
		return f
	}

	t.Run("a whitelisted reader may react", func(t *testing.T) {
		f := newPrivateFixture(t)

		assertStatus(t, f.react(http.MethodPut, commentReader, "", "👍"), http.StatusOK)
		assertStatus(t, f.list(commentReader), http.StatusOK)
	})

	t.Run("a stranger sees the post's own 404", func(t *testing.T) {
		f := newPrivateFixture(t)

		assertStatus(t, f.react(http.MethodPut, "stranger", "", "👍"), http.StatusNotFound)
		assertStatus(t, f.list("stranger"), http.StatusNotFound)

		if len(f.reactions.reactions) != 0 {
			t.Error("a reaction was stored on a post the caller cannot read")
		}
	})
}

// A moderated comment does not survive as a row of numbers.
func TestDeleteComment_TakesItsReactionsWithIt(t *testing.T) {
	f := newReactionFixture(t)
	assertStatus(t, f.react(http.MethodPut, commentReader, reactionCommentID, "🔥"), http.StatusOK)
	assertStatus(t, f.react(http.MethodPut, commentReader, "", "👍"), http.StatusOK)

	req := httptest.NewRequest(http.MethodDelete, "/blogs/"+commentSlug+"/comments/"+reactionCommentID, nil)
	req.SetPathValue("slug", commentSlug)
	req.SetPathValue("id", reactionCommentID)
	rec := httptest.NewRecorder()
	f.service.DeleteComment(rec, withUID(req, commentReader))
	assertStatus(t, rec, http.StatusNoContent)

	for _, reaction := range f.reactions.reactions {
		if reaction.Target.IsComment() {
			t.Errorf("reaction %+v outlived the comment it was on", reaction)
		}
	}
	// Only the comment's reactions go: the post's are nothing to do with it.
	if len(f.reactions.reactions) != 1 {
		t.Errorf("stored = %+v, want the post's own reaction untouched", f.reactions.reactions)
	}
}

// Cleaning up reactions is best-effort: the caller asked for the comment to be gone, and it is.
func TestDeleteComment_SucceedsWhenReactionCleanupFails(t *testing.T) {
	f := newReactionFixture(t)
	assertStatus(t, f.react(http.MethodPut, commentReader, reactionCommentID, "🔥"), http.StatusOK)
	f.reactions.deleteTargetErr = errors.New("firestore is down")

	req := httptest.NewRequest(http.MethodDelete, "/blogs/"+commentSlug+"/comments/"+reactionCommentID, nil)
	req.SetPathValue("slug", commentSlug)
	req.SetPathValue("id", reactionCommentID)
	rec := httptest.NewRecorder()
	f.service.DeleteComment(rec, withUID(req, commentReader))

	// The comment is gone even though its reactions could not be cleaned up: reporting a 500 would
	// have the caller retry a delete that already succeeded.
	assertStatus(t, rec, http.StatusNoContent)
	if _, err := f.comments.Get(t.Context(), commentSlug, reactionCommentID); err == nil {
		t.Error("the comment survived a failed reaction cleanup")
	}
}

func TestGetReactions_RepositoryFailure(t *testing.T) {
	f := newReactionFixture(t)
	f.reactions.listErr = errors.New("firestore is down")

	rec := f.list("")
	assertStatus(t, rec, http.StatusInternalServerError)
	decodeAPIError(t, rec)
}
