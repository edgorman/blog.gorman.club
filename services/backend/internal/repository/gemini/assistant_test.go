package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// modelServer stands in for the Gemini API, answering each call with the next scripted response and
// recording the requests it was sent. A scripted turn is written as the parts of one candidate,
// which is the only part of the response this adapter reads.
type modelServer struct {
	t         *testing.T
	responses [][]part
	requests  []generateRequest
	headers   []http.Header
	server    *httptest.Server
}

func newModelServer(t *testing.T, responses ...[]part) *modelServer {
	t.Helper()

	model := &modelServer{t: t, responses: responses}
	model.server = httptest.NewServer(http.HandlerFunc(model.handle))
	t.Cleanup(model.server.Close)
	return model
}

func (m *modelServer) handle(w http.ResponseWriter, r *http.Request) {
	var request generateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		m.t.Errorf("decode request: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	m.requests = append(m.requests, request)
	m.headers = append(m.headers, r.Header.Clone())

	if len(m.requests) > len(m.responses) {
		m.t.Errorf("model called %d times, only %d responses scripted", len(m.requests), len(m.responses))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var response generateResponse
	response.Candidates = append(response.Candidates, struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	}{Content: content{Role: roleModel, Parts: m.responses[len(m.requests)-1]}, FinishReason: "STOP"})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		m.t.Errorf("encode response: %v", err)
	}
}

// assistant points a configured Assistant at the fake model, with a plain client so no credentials
// are resolved.
func (m *modelServer) assistant() *Assistant {
	return NewAssistant(Config{
		Model:      "gemini-test",
		ProjectID:  "test-project",
		BaseURL:    m.server.URL,
		HTTPClient: m.server.Client(),
	})
}

func call(name string, args map[string]any) part {
	encoded, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	return part{FunctionCall: &functionCall{Name: name, Args: encoded}}
}

// A turn with nothing to edit is one round trip and leaves the draft alone.
func TestAssistant_ReplyWithoutEdits(t *testing.T) {
	model := newModelServer(t, []part{{Text: "The intro already reads well."}})
	draft := entity.Draft{Title: "Hello", Content: "body"}

	reply, err := model.assistant().Reply(context.Background(), repository.AssistantRequest{
		Draft:   draft,
		Message: "is the intro ok?",
	})
	if err != nil {
		t.Fatalf("Reply = %v, want no error", err)
	}

	if reply.Text != "The intro already reads well." {
		t.Errorf("Text = %q", reply.Text)
	}
	if len(reply.Edits) != 0 {
		t.Errorf("Edits = %+v, want none", reply.Edits)
	}
	if reply.Draft != draft {
		t.Errorf("Draft = %+v, want it unchanged", reply.Draft)
	}
	if len(model.requests) != 1 {
		t.Errorf("model called %d times, want 1", len(model.requests))
	}
}

// A tool call is applied to the draft and answered, so the model gets a second round to say what
// it did. The edited draft comes back as a value: nothing here writes it anywhere.
func TestAssistant_ReplyAppliesEdits(t *testing.T) {
	model := newModelServer(t,
		[]part{call(toolSetTitle, map[string]any{"title": "A better title"})},
		[]part{{Text: "Retitled it."}},
	)

	reply, err := model.assistant().Reply(context.Background(), repository.AssistantRequest{
		Draft:   entity.Draft{Title: "Hello", Content: "body"},
		Message: "give it a better title",
	})
	if err != nil {
		t.Fatalf("Reply = %v, want no error", err)
	}

	if reply.Draft.Title != "A better title" {
		t.Errorf("Title = %q, want the edited title", reply.Draft.Title)
	}
	if reply.Draft.Content != "body" {
		t.Errorf("Content = %q, want it untouched", reply.Draft.Content)
	}
	if len(reply.Edits) != 1 || reply.Edits[0].Tool != toolSetTitle {
		t.Fatalf("Edits = %+v, want one %s", reply.Edits, toolSetTitle)
	}
	if reply.Edits[0].Summary == "" {
		t.Error("Summary is empty, want a line describing the change")
	}
	if reply.Text != "Retitled it." {
		t.Errorf("Text = %q", reply.Text)
	}

	// The second request has to carry the model's own turn and then the tool's result, or the
	// result answers a call the transcript never shows being made.
	if len(model.requests) != 2 {
		t.Fatalf("model called %d times, want 2", len(model.requests))
	}
	second := model.requests[1].Contents
	if len(second) != 3 {
		t.Fatalf("second request had %d turns, want 3", len(second))
	}
	if second[1].Role != roleModel || second[1].Parts[0].FunctionCall == nil {
		t.Errorf("turn 2 = %+v, want the model's function call", second[1])
	}
	if second[2].Parts[0].FunctionResponse == nil {
		t.Errorf("turn 3 = %+v, want the tool result", second[2])
	}
	// The instructions are rebuilt each round, so the second round sees the edited draft.
	if !strings.Contains(model.requests[1].SystemInstruction.Parts[0].Text, "A better title") {
		t.Error("the second round's instructions do not carry the edited title")
	}
}

// A failed call is a turn in the conversation rather than a failed request: the model is told why
// and gets another round to correct itself.
func TestAssistant_ReplyRecoversFromBadCall(t *testing.T) {
	model := newModelServer(t,
		[]part{call(toolReplaceText, map[string]any{"find": "not in the post", "replace": "x"})},
		[]part{call(toolReplaceText, map[string]any{"find": "cat", "replace": "dog"})},
		[]part{{Text: "Fixed."}},
	)

	reply, err := model.assistant().Reply(context.Background(), repository.AssistantRequest{
		Draft:   entity.Draft{Title: "Hello", Content: "the cat sat"},
		Message: "say dog instead",
	})
	if err != nil {
		t.Fatalf("Reply = %v, want no error", err)
	}

	if reply.Draft.Content != "the dog sat" {
		t.Errorf("Content = %q, want the successful replacement", reply.Draft.Content)
	}
	if len(reply.Edits) != 1 {
		t.Errorf("Edits = %+v, want only the call that succeeded to be recorded", reply.Edits)
	}

	// The failed call's result has to say what went wrong, or the retry is a guess.
	result := model.requests[1].Contents[2].Parts[0].FunctionResponse
	if result == nil {
		t.Fatal("no tool result was sent back for the failed call")
	}
	if text, _ := result.Response["result"].(string); !strings.HasPrefix(text, "error:") {
		t.Errorf("result = %q, want it to report the error", text)
	}
}

// A model that only ever calls tools is stopped rather than left to loop: every round is another
// billed request carrying the whole conversation.
func TestAssistant_ReplyBoundsToolRounds(t *testing.T) {
	responses := make([][]part, maxToolRounds)
	for i := range responses {
		responses[i] = []part{call(toolSetTitle, map[string]any{"title": strings.Repeat("a", i+1)})}
	}
	model := newModelServer(t, responses...)

	if _, err := model.assistant().Reply(context.Background(), repository.AssistantRequest{
		Draft:   entity.Draft{Title: "Hello"},
		Message: "keep going",
	}); err != nil {
		t.Fatalf("Reply = %v, want no error", err)
	}

	if len(model.requests) != maxToolRounds {
		t.Errorf("model called %d times, want it stopped at %d", len(model.requests), maxToolRounds)
	}
}

// The stored history is replayed as the conversation, with the assistant's edits shown as text -
// a silent turn would read as a request nobody answered.
func TestAssistant_ReplySendsHistory(t *testing.T) {
	model := newModelServer(t, []part{{Text: "ok"}})

	if _, err := model.assistant().Reply(context.Background(), repository.AssistantRequest{
		Draft: entity.Draft{Title: "Hello"},
		History: []entity.ChatMessage{
			{Role: entity.ChatRoleUser, Content: "shorten it"},
			{Role: entity.ChatRoleAssistant, Edits: []entity.ChatEdit{{Tool: toolSetContent, Summary: "Rewrote the post"}}},
		},
		Message: "now the title",
	}); err != nil {
		t.Fatalf("Reply = %v, want no error", err)
	}

	contents := model.requests[0].Contents
	if len(contents) != 3 {
		t.Fatalf("sent %d turns, want 3", len(contents))
	}
	if contents[0].Role != roleUser || contents[0].Parts[0].Text != "shorten it" {
		t.Errorf("turn 1 = %+v", contents[0])
	}
	if contents[1].Role != roleModel || !strings.Contains(contents[1].Parts[0].Text, "Rewrote the post") {
		t.Errorf("turn 2 = %+v, want the silent turn replayed as its edits", contents[1])
	}
	if contents[2].Parts[0].Text != "now the title" {
		t.Errorf("turn 3 = %+v, want the new message last", contents[2])
	}
}

// A deployment with no model configured says so, in the same way an unconfigured verifier does,
// rather than failing as though the provider were down.
func TestAssistant_ReplyUnconfigured(t *testing.T) {
	_, err := NewAssistant(Config{ProjectID: "test-project"}).
		Reply(context.Background(), repository.AssistantRequest{Message: "hello"})

	if !errors.Is(err, repository.ErrAssistantNotConfigured) {
		t.Errorf("Reply with no model = %v, want ErrAssistantNotConfigured", err)
	}
}

// The provider's own message is never forwarded: it can quote the request, and the request holds
// the post.
func TestAssistant_ReplyProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"quota exceeded for project, prompt was: secret"}}`))
	}))
	t.Cleanup(server.Close)

	assistant := NewAssistant(Config{
		Model: "gemini-test", ProjectID: "test-project",
		BaseURL: server.URL, HTTPClient: server.Client(),
	})

	_, err := assistant.Reply(context.Background(), repository.AssistantRequest{Message: "hello"})

	if err == nil {
		t.Fatal("Reply = nil, want an error")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error = %q, want the provider's message not to be forwarded", err)
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error = %q, want the status", err)
	}
}

// The Gemini API is one global host: the model is named in the path and the project is not there
// at all - it rides in the quota header instead.
func TestAssistant_Endpoint(t *testing.T) {
	got := NewAssistant(Config{Model: "gemini-3.7-flash", ProjectID: "p"}).endpoint()

	want := "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.7-flash:generateContent"
	if got != want {
		t.Errorf("endpoint() = %q, want %q", got, want)
	}
}

// A token-authorized request carries no project of its own, so the billing project is named in a
// header. Without it the request has nothing to charge and is refused.
func TestAssistant_SendsTheQuotaProject(t *testing.T) {
	model := newModelServer(t, []part{{Text: "ok"}})

	if _, err := model.assistant().Reply(context.Background(), repository.AssistantRequest{
		Draft:   entity.Draft{Title: "Hello"},
		Message: "hello",
	}); err != nil {
		t.Fatalf("Reply = %v, want no error", err)
	}

	if got := model.headers[0].Get(quotaProjectHeader); got != "test-project" {
		t.Errorf("%s = %q, want %q", quotaProjectHeader, got, "test-project")
	}
}

// A deployment that has not named a project sends no header at all, rather than an empty one the
// API would reject outright.
func TestAssistant_OmitsAnUnsetQuotaProject(t *testing.T) {
	model := newModelServer(t, []part{{Text: "ok"}})
	assistant := NewAssistant(Config{
		Model: "gemini-test", BaseURL: model.server.URL, HTTPClient: model.server.Client(),
	})

	if _, err := assistant.Reply(context.Background(), repository.AssistantRequest{Message: "hello"}); err != nil {
		t.Fatalf("Reply = %v, want no error", err)
	}

	if _, sent := model.headers[0][http.CanonicalHeaderKey(quotaProjectHeader)]; sent {
		t.Errorf("%s was sent with no project configured", quotaProjectHeader)
	}
}
