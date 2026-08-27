package firestore

import (
	"reflect"
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
		ID:             "blog-1",
		OwnerID:        "owner",
		Title:          "Hello",
		Content:        "Body",
		Visibility:     entity.VisibilityPrivate,
		AllowedUserIDs: []string{"friend"},
		CreatedAt:      created,
		UpdatedAt:      updated,
	}

	got := blogToDocument(blog).toEntity(blog.ID)

	if !reflect.DeepEqual(got, blog) {
		t.Errorf("round trip = %+v, want %+v", got, blog)
	}
}

func TestUserMappingRoundTrip(t *testing.T) {
	user := entity.User{
		ID:          "user-1",
		Username:    "sly-dancing-monkey",
		DisplayName: "Ed",
		Bio:         "hello",
		CreatedAt:   created,
		UpdatedAt:   updated,
	}

	got := userToDocument(user).toEntity(user.ID)

	if !reflect.DeepEqual(got, user) {
		t.Errorf("round trip = %+v, want %+v", got, user)
	}
}

// The document body never carries the ID: Firestore keeps it as the document key.
func TestDocumentsExcludeID(t *testing.T) {
	for _, tt := range []struct {
		name     string
		document any
	}{
		{"blog", blogToDocument(entity.Blog{ID: "blog-1"})},
		{"user", userToDocument(entity.User{ID: "user-1"})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := reflect.TypeOf(tt.document).FieldByName("ID"); ok {
				t.Error("document type has an ID field, which would duplicate the document key")
			}
		})
	}
}
