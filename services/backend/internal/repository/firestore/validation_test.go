package firestore

import (
	"context"
	"errors"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// Each repository is built with a nil collection, which panics on any Firestore call. Returning a
// clean validation error instead therefore proves the write was refused before the datastore was
// touched, rather than merely that it was refused.
func TestRepositoriesRejectInvalidEntitiesBeforeWriting(t *testing.T) {
	ctx := context.Background()

	t.Run("blog create", func(t *testing.T) {
		_, err := (&BlogRepository{}).Create(ctx, entity.Blog{Visibility: entity.VisibilityPublic})
		assertValidationError(t, err)
	})

	t.Run("blog update", func(t *testing.T) {
		_, err := (&BlogRepository{}).Update(ctx, entity.Blog{Slug: "b1", Visibility: entity.VisibilityPublic})
		assertValidationError(t, err)
	})

	t.Run("blog with a bad visibility", func(t *testing.T) {
		_, err := (&BlogRepository{}).Create(ctx, entity.Blog{Slug: "b1", OwnerID: "owner", Visibility: "everyone"})
		assertValidationError(t, err)
	})

	// The slug is half of what names the document, so a write missing one has nowhere to go - and
	// Firestore panics rather than errors when asked for a document at an empty path.
	t.Run("blog with no slug", func(t *testing.T) {
		_, err := (&BlogRepository{}).Create(ctx, entity.Blog{OwnerID: "owner", Visibility: entity.VisibilityPublic})
		assertValidationError(t, err)
	})

	t.Run("blog with a malformed slug", func(t *testing.T) {
		_, err := (&BlogRepository{}).Create(ctx, entity.Blog{Slug: "hello world", OwnerID: "owner", Visibility: entity.VisibilityPublic})
		assertValidationError(t, err)
	})

	// A chat is keyed by the slug of its post, so the same rule applies: a write with no slug, or
	// one Firestore could not hold in a document path, has nowhere to go.
	t.Run("chat with no slug", func(t *testing.T) {
		_, err := (&ChatRepository{}).Append(ctx, "", "owner", entity.ChatMessage{Role: entity.ChatRoleUser, Content: "hi"})
		assertValidationError(t, err)
	})

	t.Run("chat with a malformed slug", func(t *testing.T) {
		_, err := (&ChatRepository{}).Append(ctx, "hello world", "owner", entity.ChatMessage{Role: entity.ChatRoleUser, Content: "hi"})
		assertValidationError(t, err)
	})

	t.Run("chat with no owner", func(t *testing.T) {
		_, err := (&ChatRepository{}).Append(ctx, "hello-world", "", entity.ChatMessage{Role: entity.ChatRoleUser, Content: "hi"})
		assertValidationError(t, err)
	})

	// A turn is validated before the transaction too, so a bad message costs no round trip.
	t.Run("chat with an empty turn", func(t *testing.T) {
		_, err := (&ChatRepository{}).Append(ctx, "hello-world", "owner", entity.ChatMessage{Role: entity.ChatRoleUser})
		assertValidationError(t, err)
	})

	// A comment lives beneath its post, so the same rule reaches it: a write naming no post, or one
	// Firestore could not hold in a document path, has nowhere to go.
	t.Run("comment with no slug", func(t *testing.T) {
		_, err := (&CommentRepository{}).Create(ctx, entity.Comment{AuthorID: "reader", Body: "nicely put"})
		assertValidationError(t, err)
	})

	t.Run("comment with a malformed slug", func(t *testing.T) {
		_, err := (&CommentRepository{}).Create(ctx, entity.Comment{BlogSlug: "hello world", AuthorID: "reader", Body: "nicely put"})
		assertValidationError(t, err)
	})

	t.Run("comment with no author", func(t *testing.T) {
		_, err := (&CommentRepository{}).Create(ctx, entity.Comment{BlogSlug: "hello-world", Body: "nicely put"})
		assertValidationError(t, err)
	})

	t.Run("comment with no body", func(t *testing.T) {
		_, err := (&CommentRepository{}).Create(ctx, entity.Comment{BlogSlug: "hello-world", AuthorID: "reader"})
		assertValidationError(t, err)
	})

	t.Run("user put", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{Username: "sly-dancing-monkey"})
		assertValidationError(t, err)
	})

	t.Run("user with no username", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{ID: "u1"})
		assertValidationError(t, err)
	})

	t.Run("user with a malformed username", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{ID: "u1", Username: "sly dancing monkey"})
		assertValidationError(t, err)
	})

	t.Run("user with a reserved username", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{ID: "u1", Username: "me"})
		assertValidationError(t, err)
	})
}

func assertValidationError(t *testing.T, err error) {
	t.Helper()

	var invalid entity.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want an entity.ValidationError", err)
	}
	if invalid.Field == "" {
		t.Error("ValidationError does not name a field")
	}
}

// A comment is addressed by the pair (blogSlug, id), so a read or a delete naming either half
// badly is a miss rather than a request Firestore is asked to parse - which it would answer by
// panicking on the path, not by erroring.
func TestCommentRepositoryRefusesUnaddressableComments(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct{ name, blogSlug, id string }{
		{"no slug", "", "abc123"},
		{"malformed slug", "hello world", "abc123"},
		{"no id", "hello-world", ""},
		{"malformed id", "hello-world", "abc/123"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := (&CommentRepository{}).Get(ctx, tt.blogSlug, tt.id); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("Get = %v, want ErrNotFound", err)
			}
			if err := (&CommentRepository{}).Delete(ctx, tt.blogSlug, tt.id); !errors.Is(err, repository.ErrNotFound) {
				t.Errorf("Delete = %v, want ErrNotFound", err)
			}
		})
	}
}

// Listing has no id to fall back on, so an unaddressable post is a validation error rather than an
// empty thread: nothing asked for a thread that could not exist.
func TestCommentRepositoryRefusesToListAnUnaddressablePost(t *testing.T) {
	ctx := context.Background()

	for _, slug := range []string{"", "hello world"} {
		t.Run(slug, func(t *testing.T) {
			_, err := (&CommentRepository{}).List(ctx, slug)
			assertValidationError(t, err)
		})
	}
}
