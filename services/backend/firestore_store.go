package main

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const blogsCollection = "blogs"

type firestoreBlogStore struct {
	client *firestore.Client
}

func newFirestoreBlogStore(client *firestore.Client) *firestoreBlogStore {
	return &firestoreBlogStore{client: client}
}

func (s *firestoreBlogStore) collection() *firestore.CollectionRef {
	return s.client.Collection(blogsCollection)
}

func (s *firestoreBlogStore) Get(ctx context.Context, id string) (Blog, error) {
	doc, err := s.collection().Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return Blog{}, ErrNotFound
	}
	if err != nil {
		return Blog{}, err
	}

	var blog Blog
	if err := doc.DataTo(&blog); err != nil {
		return Blog{}, err
	}
	blog.ID = doc.Ref.ID
	return blog, nil
}

func (s *firestoreBlogStore) Create(ctx context.Context, blog Blog) (Blog, error) {
	now := time.Now().UTC()
	blog.CreatedAt = now
	blog.UpdatedAt = now

	ref := s.collection().NewDoc()
	if _, err := ref.Set(ctx, blog); err != nil {
		return Blog{}, err
	}
	blog.ID = ref.ID
	return blog, nil
}

func (s *firestoreBlogStore) Update(ctx context.Context, blog Blog) (Blog, error) {
	blog.UpdatedAt = time.Now().UTC()

	if _, err := s.collection().Doc(blog.ID).Set(ctx, blog); err != nil {
		return Blog{}, err
	}
	return blog, nil
}

func (s *firestoreBlogStore) Delete(ctx context.Context, id string) error {
	_, err := s.collection().Doc(id).Delete(ctx)
	return err
}
