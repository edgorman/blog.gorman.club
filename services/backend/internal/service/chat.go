package service

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// assistantSilentReply stands in when the model produced neither words nor edits. A chat message
// has to say something (see entity.ChatMessage.Validate), and an empty bubble would read as the
// assistant having crashed rather than having failed to understand.
const assistantSilentReply = "I wasn't sure what to change there. Could you say it another way?"

// chatRequest is one message from the author, optionally carrying the draft they are looking at.
//
// Title and Content are pointers because omitting them and clearing them are different requests:
// omitted means "use the post as it was saved", while an explicit "" is an author who has emptied
// the field in the editor and wants the assistant to see that. They exist at all because the
// editor is a form with unsaved changes in it - asking the assistant to "tighten this paragraph"
// has to mean the paragraph on screen, not the one last written to Firestore.
type chatRequest struct {
	Message string  `json:"message"`
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

// chatResponse is a whole conversation, for a client opening the panel on a post.
type chatResponse struct {
	Messages []entity.ChatMessage `json:"messages"`
}

// chatReplyResponse is one exchange: what was said on both sides, and the post as it now stands.
//
// The post comes back whole rather than as a diff because the assistant edits the author's live
// draft: the editor has to replace what is in its fields with what was actually stored, or the
// next save would write the pre-assistant text back over it. Updated says whether that happened,
// so an editor whose author is still typing is only interrupted when there is really something to
// show them.
type chatReplyResponse struct {
	Messages []entity.ChatMessage `json:"messages"`
	Blog     blogResponse         `json:"blog"`
	Updated  bool                 `json:"updated"`
}

// requireAssistantAccess checks the caller may use the assistant at all, writing the error
// response and returning false otherwise.
//
// Access is decided from the verified credential alone, so there is nothing to read and nothing a
// request can assert about itself: the address the allowlist matches came out of a signed token
// (see entity.AssistantAllowlist). A caller who is signed in and owns the post but is not on the
// list gets a 403 saying so plainly - the feature's existence is not a secret, and there is
// nothing to hide by pretending the route is not there.
func (s *Service) requireAssistantAccess(w http.ResponseWriter, r *http.Request) bool {
	if !s.cfg.AssistantAllowlist.Allows(callerFromContext(r.Context())) {
		writeError(w, http.StatusForbidden, "the writing assistant is not enabled for your account")
		return false
	}
	return true
}

// assistantBlog resolves the post a chat route addresses and checks the caller both owns it and
// may use the assistant. Ownership is checked first so a caller who cannot see a post learns
// nothing about it from a route they are not entitled to use either.
func (s *Service) assistantBlog(w http.ResponseWriter, r *http.Request) (entity.Blog, bool) {
	blog, ok := s.requireOwnedBlog(w, r)
	if !ok {
		return entity.Blog{}, false
	}
	if !s.requireAssistantAccess(w, r) {
		return entity.Blog{}, false
	}
	return blog, true
}

// GetChat returns the conversation about a post. A post nobody has discussed answers with an empty
// conversation rather than a 404: there is nothing missing, it simply has not been started, and a
// client opening the panel would only have to translate the 404 back into the same empty list.
func (s *Service) GetChat(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.assistantBlog(w, r)
	if !ok {
		return
	}

	chat, err := s.chats.Get(r.Context(), blog.Slug)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	messages := chat.Messages
	if messages == nil {
		messages = []entity.ChatMessage{}
	}
	writeJSON(w, http.StatusOK, chatResponse{Messages: messages})
}

// DeleteChat throws the conversation away, so the author can start the assistant over without
// starting the post over. The post itself is untouched, including any edit the assistant made.
func (s *Service) DeleteChat(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.assistantBlog(w, r)
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
func draftFromRequest(blog entity.Blog, body chatRequest) (entity.Draft, error) {
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
	blog, ok := s.assistantBlog(w, r)
	if !ok {
		return
	}

	var body chatRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validated before anything is spent on it: an empty or oversized message is the author's
	// mistake, and it would be stored as a turn if the model answered it.
	asked, err := entity.NewChatMessage(entity.ChatRoleUser, body.Message)
	if err != nil {
		writeValidationError(w, err)
		return
	}

	draft, err := draftFromRequest(blog, body)
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

	writeJSON(w, http.StatusOK, chatReplyResponse{
		Messages: []entity.ChatMessage{asked, answered},
		Blog:     response,
		Updated:  updated,
	})
}
