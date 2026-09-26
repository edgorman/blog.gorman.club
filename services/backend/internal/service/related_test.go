package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// fakeEmbeddingRepository answers Nearest with a fixed ranking, whatever the vector: the index's
// ordering is Firestore's job, and what is tested here is what the service does with it.
type fakeEmbeddingRepository struct {
	stored  map[string]entity.Embedding
	nearest []string
}

func newFakeEmbeddingRepository() *fakeEmbeddingRepository {
	return &fakeEmbeddingRepository{stored: map[string]entity.Embedding{}}
}

func (r *fakeEmbeddingRepository) Get(_ context.Context, slug string) (entity.Embedding, error) {
	if e, ok := r.stored[slug]; ok {
		return e, nil
	}
	return entity.Embedding{}, repository.ErrNotFound
}

func (r *fakeEmbeddingRepository) Put(_ context.Context, e entity.Embedding) error {
	r.stored[e.Slug] = e
	return nil
}

func (r *fakeEmbeddingRepository) Delete(_ context.Context, slug string) error {
	delete(r.stored, slug)
	return nil
}

func (r *fakeEmbeddingRepository) Nearest(_ context.Context, _ []float32, limit int) ([]string, error) {
	return r.nearest[:min(limit, len(r.nearest))], nil
}

type wireRelatedPosts struct {
	Posts []wireBlog `json:"posts"`
}

func related(t *testing.T, s *Service, slug, uid string) (int, []string) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.RelatedBlogs(rec, withUID(blogPathRequest(http.MethodGet, slug, nil), uid))
	if rec.Code != http.StatusOK {
		return rec.Code, nil
	}
	var body wireRelatedPosts
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Posts must be an array on the wire, never null, so an empty list renders as nothing.
	if body.Posts == nil {
		t.Fatalf("posts is null: %s", rec.Body.String())
	}
	slugs := []string{}
	for _, p := range body.Posts {
		slugs = append(slugs, p.Slug)
	}
	return rec.Code, slugs
}

func relatedService(nearest ...string) (*Service, *fakeEmbeddingRepository) {
	blogs := newFakeBlogRepository()
	blogs.seed(
		entity.Blog{Slug: "go", OwnerID: "ed", Visibility: entity.VisibilityPublic},
		entity.Blog{Slug: "secret-go", OwnerID: "ed", Visibility: entity.VisibilityPrivate},
		entity.Blog{Slug: "rust", OwnerID: "amy", Visibility: entity.VisibilityPublic},
		entity.Blog{Slug: "zig", OwnerID: "amy", Visibility: entity.VisibilityPublic},
	)
	embeddings := newFakeEmbeddingRepository()
	embeddings.stored["go"] = entity.Embedding{Slug: "go", Vector: []float32{1}}
	embeddings.nearest = nearest

	s := newTestService(blogs, nil)
	s.embeddings = embeddings
	return s, embeddings
}

// The nearest neighbour is a private post: its owner sees it, a stranger never does, and the post
// itself is never its own neighbour.
func TestRelatedBlogs_FiltersByReadRules(t *testing.T) {
	s, _ := relatedService("go", "secret-go", "rust", "gone", "zig")

	if _, got := related(t, s, "go", ""); !slices.Equal(got, []string{"rust", "zig"}) {
		t.Errorf("anonymous = %v, want [rust zig]", got)
	}
	if _, got := related(t, s, "go", "ed"); !slices.Equal(got, []string{"secret-go", "rust", "zig"}) {
		t.Errorf("owner = %v, want [secret-go rust zig]", got)
	}
}

func TestRelatedBlogs_EmptyWithoutEmbedding(t *testing.T) {
	s, embeddings := relatedService("rust")
	delete(embeddings.stored, "go")

	code, got := related(t, s, "go", "")
	if code != http.StatusOK || len(got) != 0 {
		t.Errorf("status = %d, posts = %v, want 200 and []", code, got)
	}
}

// A caller who cannot read the post itself gets its 404, not its neighbours.
func TestRelatedBlogs_NotFoundForUnreadablePost(t *testing.T) {
	s, embeddings := relatedService("go")
	embeddings.stored["secret-go"] = entity.Embedding{Slug: "secret-go", Vector: []float32{1}}

	if code, _ := related(t, s, "secret-go", "stranger"); code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}
