package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// stubComments is a repository.CommentRepository holding at most one comment, recording what the
// worker writes onto it.
type stubComments struct {
	repository.CommentRepository
	comment *entity.Comment
	getErr  error
	written *entity.Moderation
}

func (s *stubComments) Get(_ context.Context, slug, id string) (entity.Comment, error) {
	if s.getErr != nil {
		return entity.Comment{}, s.getErr
	}
	if s.comment == nil || s.comment.BlogSlug != slug || s.comment.ID != id {
		return entity.Comment{}, repository.ErrNotFound
	}
	return *s.comment, nil
}

func (s *stubComments) SetModeration(_ context.Context, _, _ string, m entity.Moderation) error {
	s.written = &m
	return nil
}

// stubModerator answers with a fixed verdict or error, counting its calls.
type stubModerator struct {
	verdict entity.Moderation
	err     error
	calls   int
}

func (s *stubModerator) Classify(context.Context, string) (entity.Moderation, error) {
	s.calls++
	return s.verdict, s.err
}

func postComment(h http.Handler, document string) int {
	req := httpRequest("/events/comment", `{"value":{"name":"x"}}`)
	req.Header.Set("Ce-Type", "google.cloud.firestore.document.v1.created")
	req.Header.Set("Ce-Document", document)
	return serveRequest(h, req).Code
}

func TestModerateComment(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	flagged := entity.Moderation{Status: entity.ModerationFlagged, Category: "spam", Model: "gemini-test"}
	fresh := func() *entity.Comment {
		return &entity.Comment{ID: "cmt1", BlogSlug: "hello-world", AuthorID: "reader", Body: "buy now"}
	}

	t.Run("writes the classifier's verdict onto the comment", func(t *testing.T) {
		comments, model := &stubComments{comment: fresh()}, &stubModerator{verdict: flagged}
		if got := postComment(New(quiet, nil, ModerateComment(quiet, comments, model)), "blogs/hello-world/comments/cmt1"); got != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", got)
		}
		if comments.written == nil || *comments.written != flagged {
			t.Errorf("wrote %+v, want %+v", comments.written, flagged)
		}
	})

	t.Run("an already-moderated comment is skipped without a model call", func(t *testing.T) {
		comment := fresh()
		comment.Moderation = &entity.Moderation{Status: entity.ModerationApproved, Category: "none"}
		comments, model := &stubComments{comment: comment}, &stubModerator{verdict: flagged}
		if got := postComment(New(quiet, nil, ModerateComment(quiet, comments, model)), "blogs/hello-world/comments/cmt1"); got != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", got)
		}
		if model.calls != 0 || comments.written != nil {
			t.Errorf("model called %d times and wrote %+v, want neither", model.calls, comments.written)
		}
	})

	// Invalid JSON from the model surfaces as a Classify error, and is retried rather than approved.
	t.Run("a model failure is retried and writes nothing", func(t *testing.T) {
		comments, model := &stubComments{comment: fresh()}, &stubModerator{err: errors.New("gemini returned no moderation verdict")}
		if got := postComment(New(quiet, nil, ModerateComment(quiet, comments, model)), "blogs/hello-world/comments/cmt1"); got != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", got)
		}
		if comments.written != nil {
			t.Errorf("wrote %+v after a failed classification", comments.written)
		}
	})

	t.Run("a comment deleted before screening is acknowledged", func(t *testing.T) {
		comments, model := &stubComments{}, &stubModerator{verdict: flagged}
		if got := postComment(New(quiet, nil, ModerateComment(quiet, comments, model)), "blogs/hello-world/comments/cmt1"); got != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", got)
		}
		if model.calls != 0 {
			t.Errorf("model called %d times for a missing comment", model.calls)
		}
	})

	t.Run("a failed read is retried", func(t *testing.T) {
		comments, model := &stubComments{getErr: errors.New("firestore unavailable")}, &stubModerator{verdict: flagged}
		if got := postComment(New(quiet, nil, ModerateComment(quiet, comments, model)), "blogs/hello-world/comments/cmt1"); got != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", got)
		}
	})

	t.Run("a path that is not a comment is ignored", func(t *testing.T) {
		comments, model := &stubComments{comment: fresh()}, &stubModerator{verdict: flagged}
		if got := postComment(New(quiet, nil, ModerateComment(quiet, comments, model)), "blogs/hello-world"); got != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", got)
		}
		if model.calls != 0 {
			t.Errorf("model called %d times for a non-comment path", model.calls)
		}
	})
}
