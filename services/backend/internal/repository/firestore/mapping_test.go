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
		Tags:           []string{"go", "web-dev"},
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
	subscribed := updated.AddDate(0, 1, 0)
	user := entity.User{
		ID:               "user-1",
		Username:         "sly-dancing-monkey",
		Bio:              "hello",
		SubscribedUntil:  &subscribed,
		StripeCustomerID: "cus_1",
		CreatedAt:        created,
		UpdatedAt:        updated,
	}

	got := userToDocument(user).toEntity(user.ID)

	if !reflect.DeepEqual(got, user) {
		t.Errorf("round trip = %+v, want %+v", got, user)
	}
}

// An account that never subscribed stores no date at all rather than the zero time, and reads back
// as one that never subscribed rather than as one whose access ran out in 1 AD.
func TestUserMappingWithoutASubscription(t *testing.T) {
	user := entity.User{ID: "user-1", Username: "sly-dancing-monkey", CreatedAt: created, UpdatedAt: updated}

	document := userToDocument(user)

	if document.SubscribedUntil != nil {
		t.Errorf("SubscribedUntil = %v, want nothing stored", document.SubscribedUntil)
	}
	// The same for the customer: an account that has never reached a checkout has no id at the
	// payment provider, so the field is absent rather than empty.
	if document.StripeCustomerID != "" {
		t.Errorf("StripeCustomerID = %q, want nothing stored", document.StripeCustomerID)
	}
	if got := document.toEntity(user.ID); got.Subscribed(created) {
		t.Error("Subscribed = true for a profile that never subscribed, want false")
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

// A comment's id is its document key, so - like a user's, and unlike a post's slug - it is not
// stored in the body. Everything else about a comment has to survive the round trip, which is
// asserted whole so a field added to the entity but not the mapping fails here rather than
// silently stopping being persisted.
func TestCommentMappingRoundTrip(t *testing.T) {
	comment := entity.Comment{
		ID:        "cmt1",
		BlogSlug:  "hello-world",
		AuthorID:  "reader",
		Body:      "nicely put",
		CreatedAt: created,
	}

	stored := commentToDocument(comment)
	if _, ok := reflect.TypeOf(stored).FieldByName("ID"); ok {
		t.Error("comment document type has an ID field, which would duplicate the document key")
	}

	// documentToComment reads the id back off the key, which is the one thing a snapshot supplies
	// that the body does not - so it is filled in here as Firestore would.
	got := entity.Comment{ID: comment.ID, BlogSlug: stored.BlogSlug, AuthorID: stored.AuthorID, Body: stored.Body, CreatedAt: stored.CreatedAt}
	if !reflect.DeepEqual(got, comment) {
		t.Errorf("round trip = %+v, want %+v", got, comment)
	}
}

// Firestore's auto-generated ids are what a comment is keyed by, so entity.Comment.SetID has to
// admit the shape they come in - and refuse everything Firestore would not hold in a path, since
// an id read back off a URL is otherwise unqualified.
func TestCommentIDIsUsableAsADocumentKey(t *testing.T) {
	for _, id := range []string{
		"aBc123XyZ",
		strings.Repeat("a", entity.MaxCommentIDLength),
	} {
		var comment entity.Comment
		if err := comment.SetID(id); err != nil {
			t.Errorf("SetID(%q) = %v, want an id a comment can be stored at", id, err)
		}
	}

	for _, id := range []string{".", "..", "a/b", "__reserved__", "a-b", ""} {
		var comment entity.Comment
		if err := comment.SetID(id); err == nil {
			t.Errorf("SetID(%q) = nil, want an id Firestore refuses as a key to be rejected", id)
		}
	}
}

// A reaction stores what its key already says - the post, the comment, and the reader - unlike
// every other document here, which leaves the key out of the body. That is deliberate and worth
// pinning: DeleteTarget removes a comment's reactions by querying for them, and Firestore filters
// on fields rather than on the shape of a key.
func TestReactionMappingRoundTrip(t *testing.T) {
	for _, reaction := range []entity.Reaction{
		{
			Target:    entity.PostReaction("hello-world"),
			UID:       "reader",
			Emojis:    []string{"👍", "🎉"},
			UpdatedAt: updated,
		},
		{
			Target:    entity.CommentReaction("hello-world", "cmt1"),
			UID:       "reader",
			Emojis:    []string{"👎"},
			UpdatedAt: updated,
		},
	} {
		t.Run(reaction.Key(), func(t *testing.T) {
			got := reactionToDocument(reaction).toEntity()

			if !reflect.DeepEqual(got, reaction) {
				t.Errorf("round trip = %+v, want %+v", got, reaction)
			}
		})
	}
}

// The key is what makes "this reader, this target" unique, so it has to be usable as a document
// key on its own - a comment id is letters and digits (see entity.Comment.SetID), which is what
// keeps the hyphens in it unambiguous.
func TestReactionKeyIsUsableAsADocumentKey(t *testing.T) {
	for _, reaction := range []entity.Reaction{
		{Target: entity.PostReaction("hello-world"), UID: "104729"},
		{Target: entity.CommentReaction("hello-world", "aBc123"), UID: "104729"},
	} {
		key := reaction.Key()
		if key == "" || key == "." || key == ".." || strings.Contains(key, "/") {
			t.Errorf("Key() = %q, which Firestore refuses as a document key", key)
		}
	}
}
