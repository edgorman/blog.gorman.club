package main

import (
	"context"
	"slices"
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

// List runs canRead's predicate as a Firestore OR query, so a caller never fetches a private
// post they aren't on. Sorting happens here rather than in the query because ordering a
// disjunction by createdAt would need a composite index for each of its branches.
func (s *firestoreBlogStore) List(ctx context.Context, uid string) ([]Blog, error) {
	docs, err := s.collection().WhereEntity(firestore.OrFilter{
		Filters: []firestore.EntityFilter{
			firestore.PropertyFilter{Path: "visibility", Operator: "==", Value: "public"},
			firestore.PropertyFilter{Path: "ownerId", Operator: "==", Value: uid},
			firestore.PropertyFilter{Path: "allowedUserIds", Operator: "array-contains", Value: uid},
		},
	}).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	blogs := make([]Blog, 0, len(docs))
	for _, doc := range docs {
		var blog Blog
		if err := doc.DataTo(&blog); err != nil {
			return nil, err
		}
		blog.ID = doc.Ref.ID
		blogs = append(blogs, blog)
	}
	slices.SortFunc(blogs, func(a, b Blog) int { return b.CreatedAt.Compare(a.CreatedAt) })

	return blogs, nil
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
