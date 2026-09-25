package worker

import (
	"context"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// fakeBlogs serves Get from a map; the worker calls nothing else.
type fakeBlogs struct {
	repository.BlogRepository
	posts map[string]entity.Blog
}

func (f fakeBlogs) Get(_ context.Context, slug string) (entity.Blog, error) {
	if b, ok := f.posts[slug]; ok && !b.IsDeleted() {
		return b, nil
	}
	return entity.Blog{}, repository.ErrNotFound
}

type fakeEmbeddings struct {
	repository.EmbeddingRepository
	stored map[string]entity.Embedding
}

func (f fakeEmbeddings) Get(_ context.Context, slug string) (entity.Embedding, error) {
	if e, ok := f.stored[slug]; ok {
		return e, nil
	}
	return entity.Embedding{}, repository.ErrNotFound
}

func (f fakeEmbeddings) Put(_ context.Context, e entity.Embedding) error {
	f.stored[e.Slug] = e
	return nil
}

func (f fakeEmbeddings) Delete(_ context.Context, slug string) error {
	delete(f.stored, slug)
	return nil
}

type countingEmbedder struct{ calls int }

func (c *countingEmbedder) Embed(context.Context, string) ([]float32, error) {
	c.calls++
	return []float32{1, 0}, nil
}

func (c *countingEmbedder) Model() string { return "m" }

func TestEmbeddingsSync(t *testing.T) {
	post := entity.Blog{Slug: "go", OwnerID: "ed", Title: "Go", Content: "body", Visibility: entity.VisibilityPublic}
	blogs := fakeBlogs{posts: map[string]entity.Blog{"go": post}}
	store := fakeEmbeddings{stored: map[string]entity.Embedding{}}
	model := &countingEmbedder{}
	s := Embeddings{Blogs: blogs, Embeddings: store, Embedder: model}
	ctx := context.Background()
	event := Event{Document: "blogs/go"}

	// Created: embedded and stored.
	if err := s.Handle(ctx, event); err != nil {
		t.Fatal(err)
	}
	if model.calls != 1 || store.stored["go"].OwnerID != "ed" || store.stored["go"].Model != "m" {
		t.Fatalf("after create: calls=%d stored=%+v", model.calls, store.stored["go"])
	}

	// Tags and visibility only: the hash matches, so the model is not called.
	post.Tags = []string{"golang"}
	post.Visibility = entity.VisibilityPrivate
	blogs.posts["go"] = post
	if err := s.Handle(ctx, event); err != nil {
		t.Fatal(err)
	}
	if model.calls != 1 {
		t.Errorf("tag/visibility change called the model: calls=%d", model.calls)
	}

	// Content changed: re-embedded.
	post.Content = "new body"
	blogs.posts["go"] = post
	if err := s.Handle(ctx, event); err != nil {
		t.Fatal(err)
	}
	if model.calls != 2 || store.stored["go"].ContentHash != entity.ContentHash(entity.EmbeddingText(post)) {
		t.Errorf("content change: calls=%d hash=%s", model.calls, store.stored["go"].ContentHash)
	}

	// Soft-deleted: the embedding goes.
	post.DeletedAt = &post.CreatedAt
	blogs.posts["go"] = post
	if err := s.Handle(ctx, event); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.stored["go"]; ok {
		t.Error("embedding survived the post's deletion")
	}

	// Not a post document: ignored.
	if err := s.Handle(ctx, Event{Document: "blogs/go/comments/1"}); err != nil || model.calls != 2 {
		t.Errorf("comment document: err=%v calls=%d", err, model.calls)
	}
}
