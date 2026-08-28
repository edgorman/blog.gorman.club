package entity

import (
	"encoding/json"
	"testing"
	"time"
)

// The frontend's Blog/User/HealthStatus types are written against these exact field names, so a
// rename here breaks the console silently.
func TestJSONFieldNamesMatchFrontendContract(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	blog, err := json.Marshal(Blog{
		Slug: "b1", OwnerID: "o1", Title: "T", Content: "C",
		Visibility: VisibilityPublic, AllowedUserIDs: []string{"u1"},
		CreatedAt: at, UpdatedAt: at,
	})
	if err != nil {
		t.Fatalf("marshal blog: %v", err)
	}
	wantBlog := `{"slug":"b1","ownerId":"o1","title":"T","content":"C","visibility":"public","allowedUserIds":["u1"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`
	if string(blog) != wantBlog {
		t.Errorf("blog JSON =\n %s\nwant\n %s", blog, wantBlog)
	}

	// allowedUserIds is omitempty, and bio likewise, so both drop out when unset.
	bare, err := json.Marshal(Blog{Slug: "b1", OwnerID: "o1", Visibility: VisibilityPrivate, CreatedAt: at, UpdatedAt: at})
	if err != nil {
		t.Fatalf("marshal bare blog: %v", err)
	}
	wantBare := `{"slug":"b1","ownerId":"o1","title":"","content":"","visibility":"private","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`
	if string(bare) != wantBare {
		t.Errorf("bare blog JSON =\n %s\nwant\n %s", bare, wantBare)
	}

	deleted, err := json.Marshal(Blog{Slug: "b1", OwnerID: "o1", Visibility: VisibilityPrivate, CreatedAt: at, UpdatedAt: at, DeletedAt: &at})
	if err != nil {
		t.Fatalf("marshal deleted blog: %v", err)
	}
	wantDeleted := `{"slug":"b1","ownerId":"o1","title":"","content":"","visibility":"private","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","deletedAt":"2026-01-01T00:00:00Z"}`
	if string(deleted) != wantDeleted {
		t.Errorf("deleted blog JSON =\n %s\nwant\n %s", deleted, wantDeleted)
	}

	user, err := json.Marshal(User{ID: "u1", Username: "sly-dancing-monkey", Bio: "hi", CreatedAt: at, UpdatedAt: at})
	if err != nil {
		t.Fatalf("marshal user: %v", err)
	}
	wantUser := `{"id":"u1","username":"sly-dancing-monkey","bio":"hi","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`
	if string(user) != wantUser {
		t.Errorf("user JSON =\n %s\nwant\n %s", user, wantUser)
	}
}
