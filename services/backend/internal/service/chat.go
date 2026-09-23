package service

import (
	"errors"
	"log"
	"net/http"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	blogv1 "github.com/edgorman/blog.gorman.club/services/backend/internal/gen/blog/v1"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// assistantSilentReply stands in when the model produced neither words nor edits. A chat message
// has to say something (see entity.ChatMessage.Validate), and an empty bubble would read as the
// assistant having crashed rather than having failed to understand.
const assistantSilentReply = "I wasn't sure what to change there. Could you say it another way?"

// chatEditMessage is the wire shape of one change the assistant made.
func chatEditMessage(edit entity.ChatEdit) *blogv1.ChatEdit {
	return &blogv1.ChatEdit{Tool: edit.Tool, Summary: edit.Summary}
}

// chatMessageMessage is the wire shape of one turn - what GetChat's history and SendChatMessage's
// reply both carry.
func chatMessageMessage(message entity.ChatMessage) *blogv1.ChatMessage {
	edits := make([]*blogv1.ChatEdit, 0, len(message.Edits))
	for _, edit := range message.Edits {
		edits = append(edits, chatEditMessage(edit))
	}
	return &blogv1.ChatMessage{
		Role:      string(message.Role),
		Content:   message.Content,
		Edits:     edits,
		CreatedAt: timestamppb.New(message.CreatedAt),
	}
}

// chatMessagesMessage converts a run of turns the same way chatMessageMessage does, for the two
// responses - ChatHistory and ChatReply - that each carry a list of them.
func chatMessagesMessage(messages []entity.ChatMessage) []*blogv1.ChatMessage {
	converted := make([]*blogv1.ChatMessage, 0, len(messages))
	for _, message := range messages {
		converted = append(converted, chatMessageMessage(message))
	}
	return converted
}

// chatHistoryMessage is the whole conversation GetChat answers with.
func chatHistoryMessage(messages []entity.ChatMessage) *blogv1.ChatHistory {
	return &blogv1.ChatHistory{Messages: chatMessagesMessage(messages)}
}

// chatReplyMessage is one exchange: what was said on both sides, and the post as it now stands.
//
// The post comes back whole rather than as a diff because the assistant edits the author's live
// draft: the editor has to replace what is in its fields with what was actually stored, or the
// next save would write the pre-assistant text back over it. Updated says whether that happened,
// so an editor whose author is still typing is only interrupted when there is really something to
// show them.
func chatReplyMessage(messages []entity.ChatMessage, blog blogResponse, updated bool) *blogv1.ChatReply {
	return &blogv1.ChatReply{
		Messages: chatMessagesMessage(messages),
		Blog:     blogMessage(blog),
		Updated:  updated,
	}
}

// requireAssistantAccess checks the caller's account is entitled to the assistant, writing the
// error response and returning false otherwise.
//
// Nothing the request asserts about itself is consulted: the subscription comes from the caller's
// own stored profile, looked up by the uid in their verified token (see
// entity.AssistantEntitlement). A caller with no profile has no subscription either, and the zero
// profile answers that without a branch of its own - which is also why a missing one is not an
// error here.
//
// A caller who is signed in and owns the post but is not entitled gets a 403 saying so plainly:
// the feature's existence is not a secret, and there is nothing to hide by pretending the route is
// not there.
func (s *Service) requireAssistantAccess(w http.ResponseWriter, r *http.Request, action entity.Action) bool {
	uid := uidFromContext(r.Context())

	// A caller with no uid is entitled to nothing whatever a profile said, so the lookup is
	// skipped rather than made with an id no document could be at. requireAuth has already
	// refused them; this only keeps the refusal from costing a datastore read.
	var user entity.User
	if uid != "" {
		stored, err := s.users.Get(r.Context(), uid)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusInternalServerError, "internal error")
			return false
		}
		user = stored
	}

	if !s.cfg.AssistantEntitlement.Permission(action, user).Allows(uid) {
		writeError(w, http.StatusForbidden, "the writing assistant is not enabled for your account")
		return false
	}
	return true
}

// assistantBlog resolves the post a chat route addresses and checks the caller both may write it
// and is entitled to the assistant. The post is checked first so a caller who cannot see one
// learns nothing about it from a route they are not entitled to use either.
//
// The post's own permission is ActionUpdate for all three routes, whatever the route does to the
// conversation: a chat is an author's working notes on a post, so being allowed to hold one is
// being allowed to change what it is about. action is what the conversation is being asked of,
// which is the assistant's entitlement to check rather than the post's.
func (s *Service) assistantBlog(w http.ResponseWriter, r *http.Request, action entity.Action) (entity.Blog, bool) {
	blog, ok := s.requireBlogPermission(w, r, entity.ActionUpdate)
	if !ok {
		return entity.Blog{}, false
	}
	if !s.requireAssistantAccess(w, r, action) {
		return entity.Blog{}, false
	}
	return blog, true
}

// GetChat returns the conversation about a post. A post nobody has discussed answers with an empty
// conversation rather than a 404: there is nothing missing, it simply has not been started, and a
// client opening the panel would only have to translate the 404 back into the same empty list.
func (s *Service) GetChat(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.assistantBlog(w, r, entity.ActionRead)
	if !ok {
		return
	}

	chat, err := s.chats.Get(r.Context(), blog.Slug)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeProto(w, http.StatusOK, chatHistoryMessage(chat.Messages))
}

// DeleteChat throws the conversation away, so the author can start the assistant over without
// starting the post over. The post itself is untouched, including any edit the assistant made.
func (s *Service) DeleteChat(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.assistantBlog(w, r, entity.ActionDelete)
	if !ok {
		return
	}

	if err := s.chats.Delete(r.Context(), blog.Slug); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// draftFromRequest builds the draft the assistant works on: the stored post, with whatever the
// author has unsaved in front of them applied over it. Both fields go through the draft's setters,
// so a body the post itself would reject is refused here rather than reaching the model.
func draftFromRequest(blog entity.Blog, body *blogv1.ChatRequest) (entity.Draft, error) {
	draft := entity.DraftOf(blog)
	if body.Title != nil {
		if err := draft.SetTitle(*body.Title); err != nil {
			return entity.Draft{}, err
		}
	}
	if body.Content != nil {
		if err := draft.SetContent(*body.Content); err != nil {
			return entity.Draft{}, err
		}
	}
	return draft, nil
}

// SendChatMessage runs one turn of the conversation: the author's message goes to the model along
// with the draft and the history, and whatever the model edits is written back to the post.
//
// The post is written before the conversation is, and both only after the model has answered.
// Nothing is stored for a turn that failed, so a request the model never answered leaves the post
// as it was and the author's message still in the box to send again.
func (s *Service) SendChatMessage(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.assistantBlog(w, r, entity.ActionUpdate)
	if !ok {
		return
	}

	var body blogv1.ChatRequest
	if err := readProto(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validated before anything is spent on it: an empty or oversized message is the author's
	// mistake, and it would be stored as a turn if the model answered it.
	asked, err := entity.NewChatMessage(entity.ChatRoleUser, body.GetMessage())
	if err != nil {
		writeValidationError(w, err)
		return
	}

	draft, err := draftFromRequest(blog, &body)
	if err != nil {
		writeValidationError(w, err)
		return
	}

	chat, err := s.chats.Get(r.Context(), blog.Slug)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	reply, err := s.assistant.Reply(r.Context(), repository.AssistantRequest{
		Draft:   draft,
		History: chat.Messages,
		Message: asked.Content,
	})
	if errors.Is(err, repository.ErrAssistantNotConfigured) {
		writeError(w, http.StatusServiceUnavailable, "the writing assistant is not available")
		return
	}
	if err != nil {
		// Logged rather than returned: the reason is an operator's to read, and it can quote the
		// request - which holds the post - back at whoever asked.
		log.Printf("assistant reply for %q failed: %v", blog.Slug, err)
		writeError(w, http.StatusBadGateway, "the writing assistant could not be reached")
		return
	}

	// An edit that leaves the post exactly as it was is not written: the model reporting a change
	// it did not make should not bump updatedAt or overwrite what the author has since typed.
	updated := len(reply.Edits) > 0 && reply.Draft != entity.DraftOf(blog)
	if updated {
		if err := reply.Draft.ApplyTo(&blog); err != nil {
			writeValidationError(w, err)
			return
		}
		if blog, err = s.blogs.Update(r.Context(), blog); err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	text := reply.Text
	if text == "" && len(reply.Edits) == 0 {
		text = assistantSilentReply
	}
	answered, err := entity.NewChatMessage(entity.ChatRoleAssistant, text, reply.Edits...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if _, err := s.chats.Append(r.Context(), blog.Slug, blog.OwnerID, asked, answered); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	response, err := s.withAuthor(r.Context(), blog)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeProto(w, http.StatusOK, chatReplyMessage([]entity.ChatMessage{asked, answered}, response, updated))
}
