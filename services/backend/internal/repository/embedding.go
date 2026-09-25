package repository

import (
	"context"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// EmbeddingRepository stores one embedding per post, keyed by the post's slug. It knows nothing of
// read rules: Nearest ranks every stored post, and its caller filters the result by
// entity.Blog.CanBeReadBy exactly as it would any other list of posts.
type EmbeddingRepository interface {
	// Get returns ErrNotFound if slug has no embedding.
	Get(ctx context.Context, slug string) (entity.Embedding, error)
	// Put writes the embedding at its slug, replacing any there.
	Put(ctx context.Context, embedding entity.Embedding) error
	// Delete removes slug's embedding. Deleting one that isn't there is not an error.
	Delete(ctx context.Context, slug string) error
	// Nearest returns the slugs of up to limit embeddings closest to vector by cosine distance,
	// closest first.
	Nearest(ctx context.Context, vector []float32, limit int) ([]string, error)
}

// Embedder turns text into a vector with a text-embedding model.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	// Model is the model id every vector Embed returns comes from.
	Model() string
}
