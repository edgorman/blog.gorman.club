package service

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// commentResponse is a comment as clients see it: the stored comment, plus who wrote it. It
// carries a username for the same reason a post does - a comment records its author by uid, which
// is never public, so the username is the only handle a client holds for the profile behind it.
type commentResponse struct {
	entity.Comment
	AuthorUsername string `json:"authorUsername"`
}

// commentRequest is the client-settable half of a comment: what it says, and nothing else. The
// post it is on comes from the URL, its author from the credential, and its id and timestamp from
// the server, so none of them are decoded here and then overwritten.
type commentRequest struct {
	Body string `json:"body"`
}

// withCommentAuthors pairs every comment with its author's username, resolving each distinct
// commenter once however many comments they left.
func (s *Service) withCommentAuthors(r *http.Request, comments []entity.Comment) ([]commentResponse, error) {
	uids := make([]string, 0, len(comments))
	for _, comment := range comments {
		uids = append(uids, comment.AuthorID)
	}

	usernames, err := s.usernamesFor(r.Context(), uids)
	if err != nil {
		return nil, err
	}

	responses := make([]commentResponse, 0, len(comments))
	for _, comment := range comments {
		responses = append(responses, commentResponse{Comment: comment, AuthorUsername: usernames[comment.AuthorID]})
	}
	return responses, nil
}

// commentFromPath loads the comment named by the {id} it is addressed by, beneath the post it
// hangs off. A malformed id answers as a 404 for exactly the reason blogFromPath gives one for a
// malformed slug: the path is a link somebody followed, not a form they filled in.
func (s *Service) commentFromPath(w http.ResponseWriter, r *http.Request, blog entity.Blog) (entity.Comment, bool) {
	notFound := func() (entity.Comment, bool) {
		writeError(w, http.StatusNotFound, "comment not found")
		return entity.Comment{}, false
	}

	var candidate entity.Comment
	if err := candidate.SetID(r.PathValue("id")); err != nil {
		return notFound()
	}

	comment, err := s.comments.Get(r.Context(), blog.Slug, candidate.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return notFound()
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return entity.Comment{}, false
	}
	return comment, true
}

// ListComments returns the thread on a post, oldest first. It is readable by exactly whoever may
// read the post: a private post's comments are as private as the post, and a caller who cannot see
// one is answered with the same 404 the post itself gives rather than an empty thread, which would
// admit the post exists.
func (s *Service) ListComments(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}

	comments, err := s.comments.List(r.Context(), blog.Slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// withCommentAuthors always builds its slice, so a post nobody has commented on answers with an
	// empty JSON array rather than null.
	responses, err := s.withCommentAuthors(r, comments)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, responses)
}

// CreateComment adds a comment to a post, authored by the caller.
//
// Anyone who may read the post may comment on it, the owner included - which for a private post is
// its whitelist and nobody else. Commenting requires a credential either way: a comment is signed
// by whoever wrote it, and an anonymous one would be attributable to nobody and moderable only by
// deleting it.
func (s *Service) CreateComment(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}

	var body commentRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validated before the profile below is created, so an empty or oversized comment does not
	// leave a profile behind for a caller who never successfully said anything.
	comment, err := entity.NewComment(blog.Slug, uidFromContext(r.Context()), body.Body)
	if err != nil {
		writeValidationError(w, err)
		return
	}

	// A commenter is shown by username exactly as an author is, so one is assigned here for the
	// same reason publishing assigns one: a comment by a caller with no profile would be
	// attributed to nobody.
	if err := s.ensureAuthor(r.Context(), comment.AuthorID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	created, err := s.comments.Create(r.Context(), comment)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	responses, err := s.withCommentAuthors(r, []entity.Comment{created})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, responses[0])
}

// DeleteComment removes a comment. Who may is decided by entity.Comment.CanBeDeletedBy: its
// author, or the owner of the post it sits under - the second being what lets an author moderate
// their own post without being able to put words in anybody's mouth, since there is no way to edit
// a comment at all.
//
// A caller who cannot read the post gets the post's own 404 (see requireReadableBlog), so a
// stranger cannot probe a private thread for which ids exist; only once the thread is visible does
// a failed check become the 403 it really is.
func (s *Service) DeleteComment(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}

	comment, ok := s.commentFromPath(w, r, blog)
	if !ok {
		return
	}

	if !comment.CanBeDeletedBy(uidFromContext(r.Context()), blog) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := s.comments.Delete(r.Context(), blog.Slug, comment.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// The reactions go with it. They are deleted after the comment rather than before, so a
	// failure here leaves rows nothing renders rather than a comment nobody can react to; and it
	// is logged rather than returned, because the caller asked for the comment to be gone and it
	// is - reporting a 500 would have them retry a delete that already succeeded.
	if err := s.reactions.DeleteTarget(r.Context(), entity.CommentReaction(blog.Slug, comment.ID)); err != nil {
		log.Printf("deleting reactions to comment %q on %q failed: %v", comment.ID, blog.Slug, err)
	}

	w.WriteHeader(http.StatusNoContent)
}
