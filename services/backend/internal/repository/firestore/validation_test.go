package firestore

import (
	"context"
	"errors"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
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
