package firestore

import (
	"context"
	"time"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// embeddingField is the vector field the index in infrastructure/env/firestore.tf is declared on.
const embeddingField = "embedding"

type embeddingDocument struct {
	OwnerID     string      `firestore:"ownerId"`
	Embedding   fs.Vector32 `firestore:"embedding"`
	ContentHash string      `firestore:"contentHash"`
	Model       string      `firestore:"model"`
	CreatedAt   time.Time   `firestore:"createdAt"`
}

var _ repository.EmbeddingRepository = (*EmbeddingRepository)(nil)

// EmbeddingRepository keeps embeddings in their own "embeddings" collection rather than as a field
// on the post: a write to blogs/{slug} would fire the worker's trigger again, and a vector on every
// post would be read by every feed page that never uses it.
type EmbeddingRepository struct {
	embeddings *fs.CollectionRef
}

// NewEmbeddingRepository returns a repository.EmbeddingRepository backed by "embeddings".
func NewEmbeddingRepository(client *fs.Client) *EmbeddingRepository {
	return &EmbeddingRepository{embeddings: client.Collection("embeddings")}
}

func (r *EmbeddingRepository) Get(ctx context.Context, slug string) (entity.Embedding, error) {
	if slug == "" {
		return entity.Embedding{}, repository.ErrNotFound
	}
	doc, err := r.embeddings.Doc(slug).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return entity.Embedding{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.Embedding{}, err
	}
	var stored embeddingDocument
	if err := doc.DataTo(&stored); err != nil {
		return entity.Embedding{}, err
	}
	return entity.Embedding{
		Slug:        slug,
		OwnerID:     stored.OwnerID,
		Vector:      stored.Embedding,
		ContentHash: stored.ContentHash,
		Model:       stored.Model,
		CreatedAt:   stored.CreatedAt,
	}, nil
}

func (r *EmbeddingRepository) Put(ctx context.Context, e entity.Embedding) error {
	_, err := r.embeddings.Doc(e.Slug).Set(ctx, embeddingDocument{
		OwnerID:     e.OwnerID,
		Embedding:   e.Vector,
		ContentHash: e.ContentHash,
		Model:       e.Model,
		CreatedAt:   e.CreatedAt,
	})
	return err
}

func (r *EmbeddingRepository) Delete(ctx context.Context, slug string) error {
	_, err := r.embeddings.Doc(slug).Delete(ctx)
	return err
}

func (r *EmbeddingRepository) Nearest(ctx context.Context, vector []float32, limit int) ([]string, error) {
	docs, err := r.embeddings.FindNearest(embeddingField, fs.Vector32(vector), limit, fs.DistanceMeasureCosine, nil).
		Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(docs))
	for _, doc := range docs {
		slugs = append(slugs, doc.Ref.ID)
	}
	return slugs, nil
}
