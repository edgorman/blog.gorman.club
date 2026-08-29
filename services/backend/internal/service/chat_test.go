package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// chatFixture is a post, its owner's profile, and the fakes behind the assistant, wired the way a
// real request finds them: the caller owns the post and holds a profile, since a post's owner
// always has one (see ensureAuthor).
type chatFixture struct {
	service   *Service
	blogs     *fakeBlogRepository
	chats     *fakeChatRepository
	assistant *fakeAssistant
}

const (
	chatSlug  = "hello-world"
	chatOwner = "caller"
	chatEmail = "ejgorman@gmail.com"
)

// newChatFixture enables the assistant for the caller unless allowed says otherwise.
func newChatFixture(t *testing.T, allowed ...string) *chatFixture {
	t.Helper()

	if allowed == nil {
		allowed = []string{chatEmail}
	}

	blogs := newFakeBlogRepository()
	blogs.seed(entity.Blog{
		Slug:       chatSlug,
		OwnerID:    chatOwner,
		Title:      "Hello",
		Content:    "the cat sat",
		Visibility: entity.VisibilityPublic,
	})

	users := newFakeUserRepository()
	users.seed(entity.User{ID: chatOwner, Username: "calm-smiling-kestrel"})

	chats := newFakeChatRepository()
	assistant := &fakeAssistant{}

	return &chatFixture{
		service:   newAssistantService(blogs, users, chats, assistant, allowed),
		blogs:     blogs,
		chats:     chats,
		assistant: assistant,
	}
}

// chatRequestFor addresses the chat the way its route does: by the slug of the post it is about,
// carrying the verified address the allowlist is keyed on.
func chatRequestFor(method string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, "/blogs/"+url.PathEscape(chatSlug)+"/chat", body)
	req.SetPathValue("slug", chatSlug)
	return withVerifiedCaller(req, chatOwner, chatEmail)
}

func chatBody(t *testing.T, body any) io.Reader {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode body: %v", err)
	}
	return bytes.NewReader(encoded)
}

func (f *chatFixture) send(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	f.service.SendChatMessage(rec, chatRequestFor(http.MethodPost, chatBody(t, body)))
	return rec
}

func decodeChatReply(t *testing.T, rec *httptest.ResponseRecorder) chatReplyResponse {
	t.Helper()

	var got chatReplyResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return got
}

// The whole exchange comes back, along with the post as it now stands - the editor has to replace
// what is in its fields, or the next save would write the pre-assistant text back over it.
func TestSendChatMessage_AppliesEdits(t *testing.T) {
	f := newChatFixture(t)
	f.assistant.reply = func(req repository.AssistantRequest) (repository.AssistantReply, error) {
		draft := req.Draft
		draft.Content = "the dog sat"
		return repository.AssistantReply{
			Text:  "Swapped the cat for a dog.",
			Draft: draft,
			Edits: []entity.ChatEdit{{Tool: "replace_text", Summary: `Replaced "cat" with "dog"`}},
		}, nil
	}

	rec := f.send(t, map[string]string{"message": "say dog instead"})

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	got := decodeChatReply(t, rec)

	if !got.Updated {
		t.Error("Updated = false, want true - the post was edited")
	}
	if got.Blog.Content != "the dog sat" {
		t.Errorf("Blog.Content = %q, want the edited body", got.Blog.Content)
	}
	stored, _ := f.blogs.stored(chatSlug)
	if stored.Content != "the dog sat" {
		t.Errorf("stored content = %q, want the edit persisted", stored.Content)
	}

	if len(got.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want the exchange on both sides", len(got.Messages))
	}
	if got.Messages[0].Role != entity.ChatRoleUser || got.Messages[0].Content != "say dog instead" {
		t.Errorf("Messages[0] = %+v, want the question", got.Messages[0])
	}
	if got.Messages[1].Role != entity.ChatRoleAssistant || len(got.Messages[1].Edits) != 1 {
		t.Errorf("Messages[1] = %+v, want the answer and its edit", got.Messages[1])
	}

	chat, err := f.chats.Get(t.Context(), chatSlug)
	if err != nil {
		t.Fatalf("stored chat: %v", err)
	}
	if len(chat.Messages) != 2 || chat.OwnerID != chatOwner {
		t.Errorf("stored chat = %+v, want both turns owned by the caller", chat)
	}
}

// The editor is a form with unsaved changes in it, so "tighten this paragraph" has to mean the
// paragraph on screen rather than the one last written to Firestore.
func TestSendChatMessage_UsesSuppliedDraft(t *testing.T) {
	f := newChatFixture(t)

	f.send(t, map[string]any{
		"message": "any thoughts?",
		"title":   "Unsaved title",
		"content": "unsaved body",
	})

	if len(f.assistant.requests) != 1 {
		t.Fatalf("assistant called %d times, want 1", len(f.assistant.requests))
	}
	draft := f.assistant.requests[0].Draft
	if draft.Title != "Unsaved title" || draft.Content != "unsaved body" {
		t.Errorf("Draft = %+v, want what the author has on screen", draft)
	}
}

// Omitting the draft means "use the post as it was saved", which is distinct from clearing it.
func TestSendChatMessage_DefaultsToStoredDraft(t *testing.T) {
	f := newChatFixture(t)

	f.send(t, map[string]string{"message": "any thoughts?"})

	draft := f.assistant.requests[0].Draft
	if draft.Title != "Hello" || draft.Content != "the cat sat" {
		t.Errorf("Draft = %+v, want the stored post", draft)
	}
}

// A turn that only answers a question leaves the post alone - and, crucially, does not save the
// unsaved draft it was shown.
func TestSendChatMessage_WithoutEditsDoesNotWrite(t *testing.T) {
	f := newChatFixture(t)
	f.assistant.reply = func(req repository.AssistantRequest) (repository.AssistantReply, error) {
		return repository.AssistantReply{Text: "It reads well.", Draft: req.Draft}, nil
	}

	rec := f.send(t, map[string]any{"message": "is it ok?", "content": "unsaved body"})

	got := decodeChatReply(t, rec)
	if got.Updated {
		t.Error("Updated = true, want false - nothing was edited")
	}
	stored, _ := f.blogs.stored(chatSlug)
	if stored.Content != "the cat sat" {
		t.Errorf("stored content = %q, want the post untouched", stored.Content)
	}
}

// A model that reports an edit it did not make must not bump updatedAt or overwrite what the
// author has since typed.
func TestSendChatMessage_NoOpEditDoesNotWrite(t *testing.T) {
	f := newChatFixture(t)
	f.assistant.reply = func(req repository.AssistantRequest) (repository.AssistantReply, error) {
		return repository.AssistantReply{
			Text:  "Done.",
			Draft: req.Draft,
			Edits: []entity.ChatEdit{{Tool: "set_content", Summary: "Rewrote the post"}},
		}, nil
	}

	got := decodeChatReply(t, f.send(t, map[string]string{"message": "rewrite it"}))

	if got.Updated {
		t.Error("Updated = true, want false - the draft came back identical")
	}
}

// The conversation so far goes to the model, so "make that shorter" can mean something.
func TestSendChatMessage_SendsHistory(t *testing.T) {
	f := newChatFixture(t)
	f.chats.seed(entity.Chat{
		BlogSlug: chatSlug,
		OwnerID:  chatOwner,
		Messages: []entity.ChatMessage{{Role: entity.ChatRoleUser, Content: "add an intro"}},
	})

	f.send(t, map[string]string{"message": "make that shorter"})

	history := f.assistant.requests[0].History
	if len(history) != 1 || history[0].Content != "add an intro" {
		t.Errorf("History = %+v, want the conversation so far", history)
	}
}

// Nothing is stored for a turn the model never answered: the post is left as it was and the
// author's message is still theirs to send again.
func TestSendChatMessage_ProviderFailureStoresNothing(t *testing.T) {
	f := newChatFixture(t)
	f.assistant.reply = func(repository.AssistantRequest) (repository.AssistantReply, error) {
		return repository.AssistantReply{}, errors.New("gemini returned 503")
	}

	rec := f.send(t, map[string]any{"message": "rewrite it", "content": "unsaved body"})

	if rec.Result().StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadGateway)
	}
	// The provider's own message can quote the request, and the request holds the post.
	if body := decodeAPIError(t, rec); strings.Contains(body.Error, "503") {
		t.Errorf("error = %q, want the provider's message not to be forwarded", body.Error)
	}
	if _, err := f.chats.Get(t.Context(), chatSlug); !errors.Is(err, repository.ErrNotFound) {
		t.Error("a chat was stored for a turn that failed")
	}
	stored, _ := f.blogs.stored(chatSlug)
	if stored.Content != "the cat sat" {
		t.Errorf("stored content = %q, want the post untouched", stored.Content)
	}
}

// A deployment with no model says so, rather than failing as though the provider were down.
func TestSendChatMessage_Unconfigured(t *testing.T) {
	f := newChatFixture(t)
	f.assistant.reply = func(repository.AssistantRequest) (repository.AssistantReply, error) {
		return repository.AssistantReply{}, repository.ErrAssistantNotConfigured
	}

	rec := f.send(t, map[string]string{"message": "rewrite it"})

	if rec.Result().StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusServiceUnavailable)
	}
}

// A silent model still has to produce something storable, or the chat shows an empty bubble.
func TestSendChatMessage_SilentReply(t *testing.T) {
	f := newChatFixture(t)
	f.assistant.reply = func(req repository.AssistantRequest) (repository.AssistantReply, error) {
		return repository.AssistantReply{Draft: req.Draft}, nil
	}

	got := decodeChatReply(t, f.send(t, map[string]string{"message": "..."}))

	if got.Messages[1].Content == "" {
		t.Error("the assistant's turn is empty, want a stand-in reply")
	}
}

func TestSendChatMessage_RejectsEmptyMessage(t *testing.T) {
	f := newChatFixture(t)

	for _, message := range []string{"", "   ", strings.Repeat("a", entity.MaxChatMessageLength+1)} {
		rec := f.send(t, map[string]string{"message": message})

		if rec.Result().StatusCode != http.StatusBadRequest {
			t.Errorf("status for %d-character message = %d, want %d",
				len(message), rec.Result().StatusCode, http.StatusBadRequest)
		}
	}
	if len(f.assistant.requests) != 0 {
		t.Error("the model was called for a message that was never valid")
	}
}

// Access is decided from the verified credential, and checked before anything is spent on the model.
func TestSendChatMessage_NotAllowed(t *testing.T) {
	f := newChatFixture(t, "somebody-else@example.com")

	rec := f.send(t, map[string]string{"message": "rewrite it"})

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if len(f.assistant.requests) != 0 {
		t.Error("the model was called for a caller who is not allowed to use it")
	}
}

// A caller who does not own the post is answered exactly as requireOwnedBlog answers them, before
// the allowlist is consulted at all - a stranger must not learn who has the assistant.
func TestSendChatMessage_NotOwner(t *testing.T) {
	f := newChatFixture(t)

	req := httptest.NewRequest(http.MethodPost, "/blogs/"+chatSlug+"/chat", chatBody(t, map[string]string{"message": "hi"}))
	req.SetPathValue("slug", chatSlug)
	rec := httptest.NewRecorder()
	f.service.SendChatMessage(rec, withVerifiedCaller(req, "stranger", chatEmail))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	if len(f.assistant.requests) != 0 {
		t.Error("the model was called for a post the caller does not own")
	}
}

// A post nobody has discussed is an empty conversation rather than a 404: nothing is missing, it
// simply has not been started.
func TestGetChat_EmptyConversation(t *testing.T) {
	f := newChatFixture(t)

	rec := httptest.NewRecorder()
	f.service.GetChat(rec, chatRequestFor(http.MethodGet, nil))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"messages":[]`) {
		t.Errorf("body = %s, want an empty array so a client can iterate it unconditionally", rec.Body.String())
	}
}

func TestGetChat_ReturnsConversation(t *testing.T) {
	f := newChatFixture(t)
	f.chats.seed(entity.Chat{
		BlogSlug: chatSlug,
		OwnerID:  chatOwner,
		Messages: []entity.ChatMessage{{Role: entity.ChatRoleUser, Content: "add an intro"}},
	})

	rec := httptest.NewRecorder()
	f.service.GetChat(rec, chatRequestFor(http.MethodGet, nil))

	var got chatResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Messages) != 1 || got.Messages[0].Content != "add an intro" {
		t.Errorf("Messages = %+v, want the stored conversation", got.Messages)
	}
}

func TestGetChat_NotAllowed(t *testing.T) {
	f := newChatFixture(t, "somebody-else@example.com")

	rec := httptest.NewRecorder()
	f.service.GetChat(rec, chatRequestFor(http.MethodGet, nil))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
}

// Starting the assistant over must not start the post over, edits included.
func TestDeleteChat(t *testing.T) {
	f := newChatFixture(t)
	f.chats.seed(entity.Chat{
		BlogSlug: chatSlug,
		OwnerID:  chatOwner,
		Messages: []entity.ChatMessage{{Role: entity.ChatRoleUser, Content: "add an intro"}},
	})

	rec := httptest.NewRecorder()
	f.service.DeleteChat(rec, chatRequestFor(http.MethodDelete, nil))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if _, err := f.chats.Get(t.Context(), chatSlug); !errors.Is(err, repository.ErrNotFound) {
		t.Error("the conversation is still stored")
	}
	if _, ok := f.blogs.stored(chatSlug); !ok {
		t.Error("the post was deleted along with the conversation")
	}
}
