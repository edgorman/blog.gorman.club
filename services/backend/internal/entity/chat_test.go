package entity

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNewChatMessage(t *testing.T) {
	message, err := NewChatMessage(ChatRoleUser, "  tighten the intro  ")
	if err != nil {
		t.Fatalf("NewChatMessage = %v, want no error", err)
	}

	if message.Content != "tighten the intro" {
		t.Errorf("Content = %q, want it trimmed", message.Content)
	}
	if message.Role != ChatRoleUser {
		t.Errorf("Role = %q, want %q", message.Role, ChatRoleUser)
	}
	if message.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want it stamped")
	}
}

// An assistant turn may be silent when it carries edits - rewriting the intro is itself an answer
// to "rewrite the intro" - but a user turn with nothing in it is a request with no request in it.
func TestChatMessage_Validate(t *testing.T) {
	edit := ChatEdit{Tool: "set_title", Summary: "Set the title"}

	for _, tt := range []struct {
		name    string
		message ChatMessage
		valid   bool
	}{
		{"user speaks", ChatMessage{Role: ChatRoleUser, Content: "hello"}, true},
		{"assistant speaks", ChatMessage{Role: ChatRoleAssistant, Content: "done"}, true},
		{"assistant edits silently", ChatMessage{Role: ChatRoleAssistant, Edits: []ChatEdit{edit}}, true},
		{"user says nothing", ChatMessage{Role: ChatRoleUser}, false},
		{"unknown role", ChatMessage{Role: "system", Content: "hello"}, false},
		{"no role", ChatMessage{Content: "hello"}, false},
		{"too long", ChatMessage{Role: ChatRoleUser, Content: strings.Repeat("a", MaxChatMessageLength+1)}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()

			if tt.valid {
				if err != nil {
					t.Fatalf("Validate = %v, want no error", err)
				}
				return
			}
			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("Validate = %v, want a ValidationError", err)
			}
		})
	}
}

func TestChat_Append(t *testing.T) {
	chat := Chat{BlogSlug: "hello", OwnerID: "owner"}

	first, err := NewChatMessage(ChatRoleUser, "one")
	if err != nil {
		t.Fatalf("NewChatMessage = %v", err)
	}
	second, err := NewChatMessage(ChatRoleAssistant, "two")
	if err != nil {
		t.Fatalf("NewChatMessage = %v", err)
	}

	if err := chat.Append(first, second); err != nil {
		t.Fatalf("Append = %v, want no error", err)
	}

	if len(chat.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(chat.Messages))
	}
	if chat.Messages[0].Content != "one" || chat.Messages[1].Content != "two" {
		t.Errorf("Messages = %+v, want them in the order they were spoken", chat.Messages)
	}
}

// A bad turn is refused without adding any of them, so one cannot half-extend a conversation.
func TestChat_AppendInvalid(t *testing.T) {
	chat := Chat{BlogSlug: "hello", OwnerID: "owner"}
	good := ChatMessage{Role: ChatRoleUser, Content: "one"}
	bad := ChatMessage{Role: ChatRoleUser}

	err := chat.Append(good, bad)

	var invalid ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("Append = %v, want a ValidationError", err)
	}
	if len(chat.Messages) != 0 {
		t.Errorf("len(Messages) = %d, want nothing appended", len(chat.Messages))
	}
}

// The history is bounded because it is stored as one Firestore document. Overflowing it drops the
// oldest turns rather than failing the write: losing the start of a long conversation is a better
// outcome than being unable to continue it.
func TestChat_AppendTrimsToBound(t *testing.T) {
	chat := Chat{BlogSlug: "hello", OwnerID: "owner"}

	for i := range MaxChatMessages + 10 {
		message, err := NewChatMessage(ChatRoleUser, fmt.Sprintf("message %d", i))
		if err != nil {
			t.Fatalf("NewChatMessage = %v", err)
		}
		if err := chat.Append(message); err != nil {
			t.Fatalf("Append = %v, want no error", err)
		}
	}

	if len(chat.Messages) != MaxChatMessages {
		t.Fatalf("len(Messages) = %d, want %d", len(chat.Messages), MaxChatMessages)
	}
	newest := fmt.Sprintf("message %d", MaxChatMessages+9)
	if got := chat.Messages[len(chat.Messages)-1].Content; got != newest {
		t.Errorf("last message = %q, want %q", got, newest)
	}
	oldestKept := fmt.Sprintf("message %d", 10)
	if got := chat.Messages[0].Content; got != oldestKept {
		t.Errorf("first message = %q, want %q - the oldest turns should be the ones dropped", got, oldestKept)
	}
}

func TestChat_Validate(t *testing.T) {
	for _, tt := range []struct {
		name  string
		chat  Chat
		valid bool
	}{
		{"complete", Chat{BlogSlug: "hello-world", OwnerID: "owner"}, true},
		{"no slug", Chat{OwnerID: "owner"}, false},
		{"no owner", Chat{BlogSlug: "hello-world"}, false},
		// A chat is keyed by its post's slug, so it has to satisfy the rule that makes a slug
		// usable as a Firestore document key.
		{"unaddressable slug", Chat{BlogSlug: "Hello World", OwnerID: "owner"}, false},
		{"reserved slug", Chat{BlogSlug: "new", OwnerID: "owner"}, false},
		{"bad turn", Chat{BlogSlug: "hello-world", OwnerID: "owner", Messages: []ChatMessage{{Role: ChatRoleUser}}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.chat.Validate()

			if tt.valid && err != nil {
				t.Fatalf("Validate = %v, want no error", err)
			}
			if !tt.valid {
				var invalid ValidationError
				if !errors.As(err, &invalid) {
					t.Fatalf("Validate = %v, want a ValidationError", err)
				}
			}
		})
	}
}

// A conversation is only ever its owner's: there is no public or shared case as there is for a post.
func TestChat_IsOwnedBy(t *testing.T) {
	chat := Chat{BlogSlug: "hello", OwnerID: "owner"}

	if !chat.IsOwnedBy("owner") {
		t.Error("IsOwnedBy(owner) = false, want true")
	}
	if chat.IsOwnedBy("stranger") {
		t.Error("IsOwnedBy(stranger) = true, want false")
	}
	// An unowned chat is nobody's, including the caller with no uid at all.
	if (Chat{}).IsOwnedBy("") {
		t.Error("zero Chat is owned by the anonymous caller, want it owned by nobody")
	}
}
