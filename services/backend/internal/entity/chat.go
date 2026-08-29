package entity

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxChatMessageLength bounds one turn of the conversation. It is far below MaxContentLength
	// because a message is an instruction about a post, not the post itself - the assistant writes
	// the body through its tools rather than by being handed it in a chat message.
	MaxChatMessageLength = 4_000
	// MaxChatMessages is how many turns a chat keeps. A chat is stored as a single Firestore
	// document, and a document holds at most a megabyte, so the history has to be bounded
	// somewhere; Chat.Append drops the oldest turns rather than failing the write, since losing
	// the start of a long conversation is a far better outcome than being unable to continue it.
	MaxChatMessages = 100
)

// ChatRole says who spoke. There is deliberately no "system" or "tool" role: the instructions the
// model is given are rebuilt from the live post on every request rather than stored, and the tool
// calls it makes are recorded as ChatEdits on the message that made them.
type ChatRole string

const (
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
)

func (r ChatRole) Valid() bool {
	return r == ChatRoleUser || r == ChatRoleAssistant
}

// ChatEdit records one change the assistant made to the post, so the conversation shows what it
// did and not only what it said. It is a description rather than a diff: the post itself is the
// record of its own contents, and a stored diff would only be a second copy to fall out of date.
type ChatEdit struct {
	// Tool is the name of the tool the model called, e.g. "set_title".
	Tool string `json:"tool"`
	// Summary is a one-line description of the change, written for the person reading the chat.
	Summary string `json:"summary"`
}

// ChatMessage is one turn.
type ChatMessage struct {
	Role    ChatRole `json:"role"`
	Content string   `json:"content"`
	// Edits is empty for a user message and for an assistant turn that only answered a question.
	Edits     []ChatEdit `json:"edits,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// NewChatMessage builds a validated turn, stamping the time it was spoken.
func NewChatMessage(role ChatRole, content string, edits ...ChatEdit) (ChatMessage, error) {
	message := ChatMessage{
		Role:      role,
		Content:   strings.TrimSpace(content),
		Edits:     edits,
		CreatedAt: time.Now().UTC(),
	}
	if err := message.Validate(); err != nil {
		return ChatMessage{}, err
	}
	return message, nil
}

// Validate reports whether the turn is in a storable state. An assistant turn may be silent when
// it carries edits - a model that answers "rewrote the intro" by simply rewriting it has still
// said something, just not in words - but a user turn with nothing in it is a request with no
// request in it.
func (m ChatMessage) Validate() error {
	if !m.Role.Valid() {
		return ValidationError{Field: "role", Message: "must be \"user\" or \"assistant\""}
	}
	if m.Content == "" && len(m.Edits) == 0 {
		return ValidationError{Field: "content", Message: "is required"}
	}
	if utf8.RuneCountInString(m.Content) > MaxChatMessageLength {
		return ValidationError{Field: "content", Message: lengthMessage(MaxChatMessageLength)}
	}
	return nil
}

// Chat is the conversation about one post: the back-and-forth that produced it, kept so the
// assistant can be asked to "make that shorter" without being told what "that" was.
//
// It is keyed by the post's slug, exactly as the post is, so there is no id to invent and no way
// for a chat to name a post that does not exist. OwnerID is stored beside the slug so the chat can
// be authorized on its own terms rather than only through the post it belongs to - a post can be
// deleted, but its conversation is still somebody's.
type Chat struct {
	BlogSlug  string        `json:"blogSlug"`
	OwnerID   string        `json:"ownerId"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

// IsOwnedBy reports whether uid may read or write this chat. Unlike a post there is no shared or
// public case: a conversation is only ever its owner's.
func (c Chat) IsOwnedBy(uid string) bool {
	return c.OwnerID != "" && c.OwnerID == uid
}

// Append adds turns and trims the history back to MaxChatMessages, keeping the newest. It rejects
// an invalid turn without adding any of them, so a bad message cannot half-extend a conversation.
func (c *Chat) Append(messages ...ChatMessage) error {
	for _, message := range messages {
		if err := message.Validate(); err != nil {
			return err
		}
	}

	appended := append(c.Messages, messages...)
	if len(appended) > MaxChatMessages {
		appended = appended[len(appended)-MaxChatMessages:]
	}

	c.Messages = appended
	return nil
}

// Validate reports whether the chat is in a storable state: it names a post and an owner, and
// every turn it holds is one Append would have accepted.
func (c Chat) Validate() error {
	// A chat is keyed by its post's slug, so it has to satisfy exactly the rule that makes a slug
	// usable as a document key - checked by borrowing the post's own setter rather than by
	// restating it.
	var candidate Blog
	if err := candidate.SetSlug(c.BlogSlug); err != nil {
		return err
	}
	if c.OwnerID == "" {
		return ValidationError{Field: "ownerId", Message: "is required"}
	}
	for _, message := range c.Messages {
		if err := message.Validate(); err != nil {
			return err
		}
	}
	return nil
}
