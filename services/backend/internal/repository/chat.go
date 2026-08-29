package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// ChatRepository persists the assistant conversation attached to a post. A chat is identified by
// that post's slug alone, exactly as the post is, so a lookup names nothing else.
//
// There is no Update: a conversation only ever grows by a turn or is thrown away whole, and
// offering a general overwrite would make it possible to rewrite what was said.
type ChatRepository interface {
	// Get returns ErrNotFound if no chat exists for blogSlug, which is what a post nobody has
	// asked the assistant about looks like.
	Get(ctx context.Context, blogSlug string) (entity.Chat, error)
	// Append adds turns to the chat for blogSlug, creating it owned by ownerID if it is the first
	// one, and returns the whole stored conversation. ownerID is only consulted on creation - an
	// existing chat keeps the owner it was created with, so a later caller cannot take one over by
	// appending to it. It rejects a turn that fails entity.ChatMessage.Validate without writing
	// anything, and trims the history to entity.MaxChatMessages (see entity.Chat.Append).
	Append(ctx context.Context, blogSlug, ownerID string, messages ...entity.ChatMessage) (entity.Chat, error)
	// Delete removes the conversation. Unlike a post it is erased rather than marked gone: a chat
	// is a scratchpad for writing a post, and "start over" has to actually mean it.
	Delete(ctx context.Context, blogSlug string) error
}
