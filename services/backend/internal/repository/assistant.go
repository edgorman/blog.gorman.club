package repository

import (
	"context"

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
	// if the deployment has no model to call.
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
}
