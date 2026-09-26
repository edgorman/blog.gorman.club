package worker

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// Embeddings keeps embeddings/{slug} in step with blogs/{slug}.
type Embeddings struct {
	Blogs      repository.BlogRepository
	Embeddings repository.EmbeddingRepository
	Embedder   repository.Embedder
}

// Handle is the Handler for a written blogs/{slug} event. The event's own document fields are not
// read: the post is loaded fresh, so a late or redelivered event syncs whatever the post is now
// rather than what it was when the event fired.
func (s Embeddings) Handle(ctx context.Context, e Event) error {
	slug, ok := strings.CutPrefix(e.Document, "blogs/")
	if !ok || slug == "" || strings.Contains(slug, "/") {
		return nil
	}
	return s.Sync(ctx, slug)
}

// Sync embeds slug's post, or removes its embedding if the post is gone or soft-deleted. It is a
// no-op when the stored embedding already covers the post's current text with the current model,
// which is what makes a write that changes only visibility or tags free, and what stops the
// worker's own write from looping (it writes embeddings/, which fires nothing).
func (s Embeddings) Sync(ctx context.Context, slug string) error {
	blog, err := s.Blogs.Get(ctx, slug)
	if errors.Is(err, repository.ErrNotFound) {
		return s.Embeddings.Delete(ctx, slug)
	}
	if err != nil {
		return err
	}

	text := entity.EmbeddingText(blog)
	hash := entity.ContentHash(text)
	existing, err := s.Embeddings.Get(ctx, slug)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if err == nil && existing.ContentHash == hash && existing.Model == s.Embedder.Model() {
		return nil
	}

	vector, err := s.Embedder.Embed(ctx, text)
	if err != nil {
		return err
	}
	return s.Embeddings.Put(ctx, entity.Embedding{
		Slug:        slug,
		OwnerID:     blog.OwnerID,
		Vector:      vector,
		ContentHash: hash,
		Model:       s.Embedder.Model(),
		CreatedAt:   time.Now().UTC(),
	})
}
