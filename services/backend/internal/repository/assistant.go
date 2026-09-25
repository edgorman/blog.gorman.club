package repository

import (
	"context"
	"fmt"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// Assistant is a model that can talk about a post and edit it. The interface is deliberately
// narrow: one turn in, one turn out, with the edited draft as a value. Everything that makes it a
// language model - prompts, tool declarations, the call-and-respond loop a model runs to make more
// than one edit - lives behind it, so swapping providers means adding a folder under repository/
// and changing one line in cmd/backend, exactly as it does for the datastore.
//
// Nothing behind this interface writes: the implementation edits a copy of the draft and hands it
// back, and the service decides whether to persist it. A model cannot reach Firestore even in
// principle.
type Assistant interface {
	// Reply answers Message in the context of the conversation so far and returns the draft as the
	// model left it, along with a record of what it changed. It returns ErrAssistantNotConfigured
	// if the deployment has no model to call, and an *AssistantStatusError when the model's API
	// answered with an error status.
	//
	// The reply's Usage is filled in whether or not the turn succeeded: a turn that failed on its
	// third round still spent the first two, and that is exactly the usage an operator watching for
	// a runaway loop needs to see. Every other field of a failed turn's reply is empty.
	Reply(ctx context.Context, req AssistantRequest) (AssistantReply, error)
}

// AssistantRequest is one turn's worth of input.
type AssistantRequest struct {
	// Draft is the post as it stands, which for a caller mid-edit is what they are looking at
	// rather than what was last saved.
	Draft entity.Draft
	// History is the conversation before this turn, oldest first.
	History []entity.ChatMessage
	// Message is what the user just said.
	Message string
}

// AssistantReply is one turn's worth of output.
type AssistantReply struct {
	// Text is what the model said, which may be empty when it only edited.
	Text string
	// Draft is the post as the model left it, unchanged from the request when it made no edits.
	Draft entity.Draft
	// Edits records the changes behind that draft, in the order they were made. It is empty
	// exactly when Draft came back untouched.
	Edits []entity.ChatEdit
	// Usage is what the turn cost, for the log line each turn writes (see service.SendChatMessage).
	Usage AssistantUsage
}

// AssistantUsage is what one turn cost: how many times the model was called, and the tokens it
// reported across all of those calls. It carries counts only, never anything that was said.
type AssistantUsage struct {
	// Rounds is how many calls to the model the turn made - one, plus one for each round of tool
	// calls it went through.
	Rounds int
	// PromptTokens, CandidateTokens and TotalTokens are the model's own counts, summed over the
	// rounds. Total can exceed the other two combined: a thinking model bills its reasoning there.
	PromptTokens    int
	CandidateTokens int
	TotalTokens     int
}

// AssistantStatusError is the model's API answering with an error status. The status is kept as a
// number rather than only folded into the message, so it can be logged as a field of its own and
// counted by it: a 429 (quota) and a 403 (a missing role) need entirely different fixes.
type AssistantStatusError struct {
	// StatusCode is the HTTP status the API answered with.
	StatusCode int
	// Detail is the API's machine-readable reason, e.g. " (RESOURCE_EXHAUSTED)", or empty. It is
	// never the API's prose message, which can quote the request - and so the post - back.
	Detail string
}

func (e *AssistantStatusError) Error() string {
	return fmt.Sprintf("model returned %d%s", e.StatusCode, e.Detail)
}
