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
		_, err := (&BlogRepository{}).Update(ctx, entity.Blog{ID: "b1", Visibility: entity.VisibilityPublic})
		assertValidationError(t, err)
	})

	t.Run("blog with a bad visibility", func(t *testing.T) {
		_, err := (&BlogRepository{}).Create(ctx, entity.Blog{OwnerID: "owner", Visibility: "everyone"})
		assertValidationError(t, err)
	})

	t.Run("user put", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{DisplayName: "Ed"})
		assertValidationError(t, err)
	})

	t.Run("user with a blank display name", func(t *testing.T) {
		_, err := (&UserRepository{}).Put(ctx, entity.User{ID: "u1", DisplayName: "  "})
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
