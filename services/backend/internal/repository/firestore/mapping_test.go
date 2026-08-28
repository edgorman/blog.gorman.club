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
// keyed by owner and slug together, which it carries as ordinary fields - the key is derived from
// them (see blogKey), so the body is what identifies the post and the key is what enforces it.
func TestUserDocumentExcludesID(t *testing.T) {
	if _, ok := reflect.TypeOf(userToDocument(entity.User{ID: "user-1"})).FieldByName("ID"); ok {
		t.Error("user document type has an ID field, which would duplicate the document key")
	}
}

// Two posts collide only when their owner and slug both match: an author may reuse a slug nobody
// else's post can take from them, and may not reuse their own.
func TestBlogKeyIsScopedToTheOwner(t *testing.T) {
	mine := blogKey("owner", "hello-world")

	if same := blogKey("owner", "hello-world"); same != mine {
		t.Errorf("blogKey is not stable: %q then %q", mine, same)
	}
	if other := blogKey("another-owner", "hello-world"); other == mine {
		t.Errorf("blogKey(%q) = %q for two owners, want one slug per author to be free of the other", "hello-world", mine)
	}
	if other := blogKey("owner", "hello-world-k3m9x"); other == mine {
		t.Errorf("blogKey collides for two of one owner's slugs at %q", mine)
	}
	// A slug cannot hold the separator, so no owner/slug pair can be spelled two ways.
	if strings.Contains(mine, blogKeySeparator+blogKeySeparator) {
		t.Errorf("blogKey = %q, want a single separator", mine)
	}
	var blog entity.Blog
	if err := blog.SetSlug("hello" + blogKeySeparator + "world"); err == nil {
		t.Errorf("SetSlug admitted the key separator %q, which would make two pairs share a key", blogKeySeparator)
	}
}
