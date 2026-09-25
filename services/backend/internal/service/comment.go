package service

import (
	"errors"
	"net/http"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	blogv1 "github.com/edgorman/blog.gorman.club/services/backend/internal/gen/blog/v1"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// commentResponse is a comment as clients see it: the stored comment, plus who wrote it. It
// carries a username for the same reason a post does - a comment records its author by uid, which
// is never public, so the username is the only handle a client holds for the profile behind it.
type commentResponse struct {
	entity.Comment
	AuthorUsername string
}

// commentMessage is the wire shape of a single comment: what CreateComment answers with, and what
// each entry of a CommentThread carries.
//
// Moderation is carried whenever the comment holds one, so a caller who may not see it has to have
// it cleared first (see ListComments).
func commentMessage(response commentResponse) *blogv1.Comment {
	message := &blogv1.Comment{
		Id:             response.ID,
		BlogSlug:       response.BlogSlug,
		AuthorId:       response.AuthorID,
		AuthorUsername: response.AuthorUsername,
		Body:           response.Body,
		CreatedAt:      timestamppb.New(response.CreatedAt),
	}
	if m := response.Moderation; m != nil {
		message.Moderation = &blogv1.CommentModeration{Status: string(m.Status), Category: m.Category}
	}
	return message
}

// commentThreadMessage is the whole thread ListComments answers with, converting each comment the
// same way commentMessage does.
func commentThreadMessage(responses []commentResponse) *blogv1.CommentThread {
	comments := make([]*blogv1.Comment, 0, len(responses))
	for _, response := range responses {
		comments = append(comments, commentMessage(response))
	}
	return &blogv1.CommentThread{Comments: comments}
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

	// A flagged comment is dropped for everybody but its author and the post's owner, and only the
	// owner is told how it was screened - its author sees it exactly as they wrote it.
	uid := uidFromContext(r.Context())
	visible := make([]entity.Comment, 0, len(comments))
	for _, comment := range comments {
		if !comment.VisibleTo(uid, blog) {
			continue
		}
		if !comment.ModerationPermission(entity.ActionRead, blog).Allows(uid) {
			comment.Moderation = nil
		}
		visible = append(visible, comment)
	}
	comments = visible

	// withCommentAuthors always builds its slice, and commentThreadMessage does the same with
	// Comments, so a post nobody has commented on answers with an empty JSON array rather than null.
	responses, err := s.withCommentAuthors(r, comments)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeProto(w, http.StatusOK, commentThreadMessage(responses))
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

	var body blogv1.CreateCommentRequest
	if err := readProto(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validated before the profile below is created, so an empty or oversized comment does not
	// leave a profile behind for a caller who never successfully said anything.
	comment, err := entity.NewComment(blog.Slug, uidFromContext(r.Context()), body.GetBody())
	if err != nil {
		writeValidationError(w, err)
		return
	}

	// A comment is written signed by whoever asked, so - like creating a post - the permission is
	// asked of a comment that already names them, and what it excludes is the caller with no uid.
	if !requirePermission(w, r, comment.Permission(entity.ActionCreate, blog)) {
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

	writeProto(w, http.StatusCreated, commentMessage(responses[0]))
}

// DeleteComment removes a comment. Who may is decided by the comment's own delete permission: its
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

	if !requirePermission(w, r, comment.Permission(entity.ActionDelete, blog)) {
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
		s.logger().ErrorContext(r.Context(), "deleting a comment's reactions failed",
			"slug", blog.Slug, "comment_id", comment.ID, "error", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// ApproveComment overrules the classifier on a comment, putting a flagged one back in its thread.
// Only the post's owner may, per the comment's moderation permission; a caller who cannot read the
// post gets its 404 first, as with DeleteComment. Approving is idempotent, so approving a comment
// that was never flagged, or never screened, simply records the owner's approval.
func (s *Service) ApproveComment(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}

	comment, ok := s.commentFromPath(w, r, blog)
	if !ok {
		return
	}

	if !requirePermission(w, r, comment.ModerationPermission(entity.ActionUpdate, blog)) {
		return
	}

	// The classifier's category is kept as the record of why it was flagged; an empty model says
	// the owner, not a model, made this call.
	approval := entity.Moderation{Status: entity.ModerationApproved, Category: "none", At: time.Now().UTC()}
	if comment.Moderation != nil {
		approval.Category = comment.Moderation.Category
	}

	err := s.comments.SetModeration(r.Context(), blog.Slug, comment.ID, approval)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	comment.Moderation = &approval

	responses, err := s.withCommentAuthors(r, []entity.Comment{comment})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeProto(w, http.StatusOK, commentMessage(responses[0]))
}
