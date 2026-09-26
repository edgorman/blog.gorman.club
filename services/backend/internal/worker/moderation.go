package worker

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// ModerateComment screens a newly created comment and writes the verdict onto it. Comments are
// published first and hidden afterwards if flagged, so every way this gives up - a malformed path,
// a comment already deleted, retries exhausted - leaves the comment visible rather than lost.
//
// The comment is read back from Firestore rather than from the event, so a redelivered event finds
// the moderation an earlier delivery wrote and stops there without calling the model.
func ModerateComment(log *slog.Logger, comments repository.CommentRepository, moderator repository.Moderator) Handler {
	return func(ctx context.Context, e Event) error {
		slug, id, ok := commentPath(e.Document)
		if !ok {
			log.WarnContext(ctx, "comment event ignored: not a comment path", "document", e.Document)
			return nil
		}

		comment, err := comments.Get(ctx, slug, id)
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if comment.Moderation != nil {
			return nil
		}

		// ponytail: a duplicate delivery racing this one can overwrite an owner's approval made in
		// between; a Firestore precondition on the read's update time closes it if that ever matters.
		moderation, err := moderator.Classify(ctx, comment.Body)
		if err != nil {
			return err
		}
		if err := comments.SetModeration(ctx, slug, id, moderation); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil
			}
			return err
		}

		log.InfoContext(ctx, "comment moderated", "document", e.Document,
			"status", string(moderation.Status), "category", moderation.Category)
		return nil
	}
}

// commentPath splits "blogs/{slug}/comments/{id}" into its slug and id.
func commentPath(document string) (slug, id string, ok bool) {
	parts := strings.Split(document, "/")
	if len(parts) != 4 || parts[0] != "blogs" || parts[2] != "comments" || parts[1] == "" || parts[3] == "" {
		return "", "", false
	}
	return parts[1], parts[3], true
}
