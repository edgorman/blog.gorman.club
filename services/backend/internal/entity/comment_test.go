package entity

import (
	"errors"
	"strings"
	"testing"
)

func TestNewComment(t *testing.T) {
	comment, err := NewComment("hello-world", "reader", "  nicely put  ")
	if err != nil {
		t.Fatalf("NewComment = %v, want no error", err)
	}

	if comment.Body != "nicely put" {
		t.Errorf("Body = %q, want it trimmed", comment.Body)
	}
	if comment.BlogSlug != "hello-world" {
		t.Errorf("BlogSlug = %q, want %q", comment.BlogSlug, "hello-world")
	}
	if comment.AuthorID != "reader" {
		t.Errorf("AuthorID = %q, want %q", comment.AuthorID, "reader")
	}
	if comment.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want it stamped")
	}
	// The store assigns the id, so a comment on its way to being written has none - and Validate
	// has to accept it in that state or nothing could ever be created.
	if comment.ID != "" {
		t.Errorf("ID = %q, want it left for the repository to assign", comment.ID)
	}
}

func TestNewComment_Invalid(t *testing.T) {
	for _, tt := range []struct {
		name     string
		blogSlug string
		authorID string
		body     string
	}{
		{"no body", "hello-world", "reader", ""},
		{"only whitespace", "hello-world", "reader", "   \n  "},
		{"too long", "hello-world", "reader", strings.Repeat("a", MaxCommentLength+1)},
		{"no post", "", "reader", "nicely put"},
		{"malformed slug", "hello world", "reader", "nicely put"},
		{"no author", "hello-world", "", "nicely put"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewComment(tt.blogSlug, tt.authorID, tt.body)

			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("NewComment = %v, want a ValidationError", err)
			}
			if invalid.Field == "" {
				t.Error("ValidationError does not name a field")
			}
		})
	}
}

// A body of exactly the limit is storable: the bound is what a comment may be, not one below it.
func TestComment_BodyAtTheLimit(t *testing.T) {
	if _, err := NewComment("hello-world", "reader", strings.Repeat("a", MaxCommentLength)); err != nil {
		t.Fatalf("NewComment = %v, want no error", err)
	}
}

// The id is what names the document a comment lives at, so it has to survive being put in a path -
// which is the same rule a slug answers to, for the same reason.
func TestComment_SetID(t *testing.T) {
	for _, tt := range []struct {
		name  string
		id    string
		valid bool
	}{
		{"firestore's own shape", "aBc123XyZ", true},
		{"trimmed", "  abc123  ", true},
		{"empty", "", false},
		{"holds a slash", "abc/def", false},
		{"a path segment", "..", false},
		{"holds a space", "abc def", false},
		{"firestore's reserved form", "__reserved__", false},
		{"too long", strings.Repeat("a", MaxCommentIDLength+1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var comment Comment
			err := comment.SetID(tt.id)

			if tt.valid {
				if err != nil {
					t.Fatalf("SetID = %v, want no error", err)
				}
				if comment.ID != strings.TrimSpace(tt.id) {
					t.Errorf("ID = %q, want %q", comment.ID, strings.TrimSpace(tt.id))
				}
				return
			}
			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("SetID = %v, want a ValidationError", err)
			}
			if comment.ID != "" {
				t.Errorf("ID = %q, want a rejected id not to be applied", comment.ID)
			}
		})
	}
}

// Deleting a comment is the one thing two different people may do, so this is the whole of the
// moderation rule: the commenter can retract what they said, the author can take it off their
// post, and nobody else - including a reader of the post - can touch it.
func TestComment_CanBeDeletedBy(t *testing.T) {
	post := Blog{Slug: "hello-world", OwnerID: "author", Visibility: VisibilityPublic}
	comment := Comment{ID: "c1", BlogSlug: "hello-world", AuthorID: "reader", Body: "nicely put"}

	for _, tt := range []struct {
		name string
		uid  string
		want bool
	}{
		{"its author", "reader", true},
		{"the post's owner", "author", true},
		{"another reader", "stranger", false},
		{"nobody", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := comment.CanBeDeletedBy(tt.uid, post); got != tt.want {
				t.Errorf("CanBeDeletedBy(%q) = %v, want %v", tt.uid, got, tt.want)
			}
		})
	}

	// An unowned post cannot hand its moderation rights to the zero uid, and a comment with no
	// author cannot be deleted by one either - both would otherwise match on "" == "".
	t.Run("an anonymous caller against an ownerless post", func(t *testing.T) {
		if (Comment{}).CanBeDeletedBy("", Blog{}) {
			t.Error("CanBeDeletedBy = true, want the zero uid to match nothing")
		}
	})
}
