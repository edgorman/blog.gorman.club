package firestore

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

var (
	created = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	updated = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
)

// A field added to entity.Blog but not to the mapping would silently stop being persisted, so the
// round trip is asserted whole rather than field by field.
func TestBlogMappingRoundTrip(t *testing.T) {
	blog := entity.Blog{
		Slug:           "hello-world",
		OwnerID:        "owner",
		Title:          "Hello",
		Content:        "Body",
		Visibility:     entity.VisibilityPrivate,
		AllowedUserIDs: []string{"friend"},
		CreatedAt:      created,
		UpdatedAt:      updated,
	}

	got := blogToDocument(blog).toEntity()

	if !reflect.DeepEqual(got, blog) {
		t.Errorf("round trip = %+v, want %+v", got, blog)
	}
}

// DeletedAt is what makes a post soft-deleted rather than gone, so the round trip must carry it
// too - a mapping that dropped it would resurrect every deleted post on its next read.
func TestBlogMappingRoundTrip_DeletedAt(t *testing.T) {
	deletedAt := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	blog := entity.Blog{
		Slug:       "hello-world",
		OwnerID:    "owner",
		Visibility: entity.VisibilityPublic,
		CreatedAt:  created,
		UpdatedAt:  updated,
		DeletedAt:  &deletedAt,
	}

	got := blogToDocument(blog).toEntity()

	if !reflect.DeepEqual(got, blog) {
		t.Errorf("round trip = %+v, want %+v", got, blog)
	}
	if !got.IsDeleted() {
		t.Error("IsDeleted() = false after round trip, want true")
	}
}

// A chat is stored as one document with its turns inside it, so the whole conversation - edits
// included - has to survive the round trip; a mapping that dropped the edits would lose the record
// of what the assistant actually did.
func TestChatMappingRoundTrip(t *testing.T) {
	chat := entity.Chat{
		BlogSlug: "hello-world",
		OwnerID:  "owner",
		Messages: []entity.ChatMessage{
			{Role: entity.ChatRoleUser, Content: "tighten the intro", CreatedAt: created},
			{
				Role:      entity.ChatRoleAssistant,
				Content:   "Done.",
				Edits:     []entity.ChatEdit{{Tool: "set_content", Summary: "Rewrote the post"}},
				CreatedAt: updated,
			},
		},
		CreatedAt: created,
		UpdatedAt: updated,
	}

	got := chatToDocument(chat).toEntity()

	if !reflect.DeepEqual(got, chat) {
		t.Errorf("round trip = %+v, want %+v", got, chat)
	}
}

// An empty conversation round-trips to an empty one rather than to a nil that reads as absent.
func TestChatMappingRoundTrip_NoMessages(t *testing.T) {
	chat := entity.Chat{BlogSlug: "hello-world", OwnerID: "owner", CreatedAt: created, UpdatedAt: updated}

	got := chatToDocument(chat).toEntity()

	if len(got.Messages) != 0 {
		t.Errorf("Messages = %+v, want none", got.Messages)
	}
	if got.BlogSlug != chat.BlogSlug || got.OwnerID != chat.OwnerID {
		t.Errorf("round trip = %+v, want %+v", got, chat)
	}
}

func TestUserMappingRoundTrip(t *testing.T) {
	user := entity.User{
		ID:        "user-1",
		Username:  "sly-dancing-monkey",
		Bio:       "hello",
		CreatedAt: created,
		UpdatedAt: updated,
	}

	got := userToDocument(user).toEntity(user.ID)

	if !reflect.DeepEqual(got, user) {
		t.Errorf("round trip = %+v, want %+v", got, user)
	}
}

// A user's id is its document key, so storing it in the body too would duplicate it. A post is
// keyed by its slug, which it also carries as an ordinary field, so the body is what identifies
// the post and the key is what enforces its uniqueness.
func TestUserDocumentExcludesID(t *testing.T) {
	if _, ok := reflect.TypeOf(userToDocument(entity.User{ID: "user-1"})).FieldByName("ID"); ok {
		t.Error("user document type has an ID field, which would duplicate the document key")
	}
}

// A post is stored at its slug alone, so the slug is the whole of what makes two posts collide:
// no two posts hold one whoever wrote them, which is what lets a post be addressed without naming
// its author. Every slug also has to survive Firestore's rules for a document key on its own,
// since nothing else qualifies it any more.
func TestBlogSlugIsUsableAsADocumentKey(t *testing.T) {
	for _, slug := range []string{
		"hello-world",
		"hello-world-k3m9x",
		"untitled",
		strings.Repeat("a", entity.MaxBlogSlugLength),
	} {
		var blog entity.Blog
		if err := blog.SetSlug(slug); err != nil {
			t.Errorf("SetSlug(%q) = %v, want a slug a post can be stored at", slug, err)
		}
	}

	// Firestore refuses these outright in a document path, and a slug is now the whole path
	// segment, so SetSlug is the only thing standing between a title and an unstorable key.
	for _, slug := range []string{".", "..", "hello/world", "__reserved__", ""} {
		var blog entity.Blog
		if err := blog.SetSlug(slug); err == nil {
			t.Errorf("SetSlug(%q) = nil, want a slug Firestore refuses as a key to be rejected", slug)
		}
	}
}
