package firestore

import (
	"context"
	"time"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// chatDocument is the stored shape of a conversation; see blogDocument for why it is separate.
//
// The turns are an array inside the one document rather than a subcollection beneath it. A chat is
// only ever read whole - the model is given the whole history on every turn - so a subcollection
// would buy pagination nothing here needs and cost a query per read. What it would buy is
// unbounded growth, which entity.MaxChatMessages gives up deliberately: a bounded history fits the
// megabyte a document holds, and the oldest turns of a hundred-turn conversation are worth less
// than the simplicity of storing it as one thing.
type chatDocument struct {
	BlogSlug  string                `firestore:"blogSlug"`
	OwnerID   string                `firestore:"ownerId"`
	Messages  []chatMessageDocument `firestore:"messages"`
	CreatedAt time.Time             `firestore:"createdAt"`
	UpdatedAt time.Time             `firestore:"updatedAt"`
}

type chatMessageDocument struct {
	Role      string             `firestore:"role"`
	Content   string             `firestore:"content"`
	Edits     []chatEditDocument `firestore:"edits"`
	CreatedAt time.Time          `firestore:"createdAt"`
}

type chatEditDocument struct {
	Tool    string `firestore:"tool"`
	Summary string `firestore:"summary"`
}

func chatToDocument(chat entity.Chat) chatDocument {
	messages := make([]chatMessageDocument, 0, len(chat.Messages))
	for _, message := range chat.Messages {
		edits := make([]chatEditDocument, 0, len(message.Edits))
		for _, edit := range message.Edits {
			edits = append(edits, chatEditDocument{Tool: edit.Tool, Summary: edit.Summary})
		}
		messages = append(messages, chatMessageDocument{
			Role:      string(message.Role),
			Content:   message.Content,
			Edits:     edits,
			CreatedAt: message.CreatedAt,
		})
	}

	return chatDocument{
		BlogSlug:  chat.BlogSlug,
		OwnerID:   chat.OwnerID,
		Messages:  messages,
		CreatedAt: chat.CreatedAt,
		UpdatedAt: chat.UpdatedAt,
	}
}

// toEntity rebuilds a conversation from its stored fields. Like a post - and unlike a user - a
// chat carries everything that identifies it in the body, so the key is never read back.
func (d chatDocument) toEntity() entity.Chat {
	messages := make([]entity.ChatMessage, 0, len(d.Messages))
	for _, stored := range d.Messages {
		var edits []entity.ChatEdit
		for _, edit := range stored.Edits {
			edits = append(edits, entity.ChatEdit{Tool: edit.Tool, Summary: edit.Summary})
		}
		messages = append(messages, entity.ChatMessage{
			Role:      entity.ChatRole(stored.Role),
			Content:   stored.Content,
			Edits:     edits,
			CreatedAt: stored.CreatedAt,
		})
	}

	return entity.Chat{
		BlogSlug:  d.BlogSlug,
		OwnerID:   d.OwnerID,
		Messages:  messages,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

var _ repository.ChatRepository = (*ChatRepository)(nil)

// ChatRepository stores each conversation at the slug of the post it is about, so a chat is keyed
// exactly as its post is and the two can never disagree about which post is being discussed. The
// slug's shape is what keeps it usable as a document key (see entity.Blog.SetSlug), and Get and
// Delete refuse an empty one rather than asking Firestore about a path it cannot parse.
type ChatRepository struct {
	client *fs.Client
	chats  *fs.CollectionRef
}

// NewChatRepository returns a repository.ChatRepository backed by the "chats" collection. The
// client is kept because Append is a read-modify-write and so has to run in a transaction.
func NewChatRepository(client *fs.Client) *ChatRepository {
	return &ChatRepository{client: client, chats: client.Collection("chats")}
}

func (r *ChatRepository) Get(ctx context.Context, blogSlug string) (entity.Chat, error) {
	if blogSlug == "" {
		return entity.Chat{}, repository.ErrNotFound
	}

	doc, err := r.chats.Doc(blogSlug).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return entity.Chat{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.Chat{}, err
	}

	var stored chatDocument
	if err := doc.DataTo(&stored); err != nil {
		return entity.Chat{}, err
	}
	return stored.toEntity(), nil
}

// Append reads the conversation and writes it back with the new turns in one transaction, rather
// than pushing them onto the stored array with ArrayUnion. Two things need that: the history is
// trimmed to a bound, which a blind append cannot do, and ArrayUnion treats an element already
// present as already appended - so asking the assistant the same question twice would silently
// lose the second turn.
func (r *ChatRepository) Append(ctx context.Context, blogSlug, ownerID string, messages ...entity.ChatMessage) (entity.Chat, error) {
	// Validated before the transaction so a bad turn costs no round trip, and so Doc is never
	// handed a path it would panic on.
	if err := (entity.Chat{BlogSlug: blogSlug, OwnerID: ownerID}).Validate(); err != nil {
		return entity.Chat{}, err
	}
	for _, message := range messages {
		if err := message.Validate(); err != nil {
			return entity.Chat{}, err
		}
	}

	now := time.Now().UTC()
	doc := r.chats.Doc(blogSlug)

	var chat entity.Chat
	err := r.client.RunTransaction(ctx, func(ctx context.Context, tx *fs.Transaction) error {
		// Rebuilt from scratch on every attempt rather than carried across them: a transaction is
		// re-run when it loses a race, and appending to what the last attempt already appended to
		// would store the new turns twice.
		chat = entity.Chat{BlogSlug: blogSlug, OwnerID: ownerID, CreatedAt: now}

		stored, err := tx.Get(doc)
		switch {
		case status.Code(err) == codes.NotFound:
			// The first turn about this post: the chat is created owned by the caller.
		case err != nil:
			return err
		default:
			var current chatDocument
			if err := stored.DataTo(&current); err != nil {
				return err
			}
			// Everything but the new turns comes from the stored chat, so an existing
			// conversation keeps the owner and creation time it was written with rather than
			// taking the caller's.
			chat = current.toEntity()
		}

		if err := chat.Append(messages...); err != nil {
			return err
		}
		chat.UpdatedAt = now

		return tx.Set(doc, chatToDocument(chat))
	})
	if err != nil {
		return entity.Chat{}, err
	}
	return chat, nil
}

// Delete erases the conversation. Firestore deletes are idempotent, so a chat that was never
// started deletes successfully - the service looks it up first when it wants to report a 404.
func (r *ChatRepository) Delete(ctx context.Context, blogSlug string) error {
	if blogSlug == "" {
		return repository.ErrNotFound
	}

	_, err := r.chats.Doc(blogSlug).Delete(ctx)
	return err
}
