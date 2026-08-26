package main

import (
	"context"
	"slices"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type firestoreBlogStore struct {
	blogs *firestore.CollectionRef
}

func newFirestoreBlogStore(client *firestore.Client) *firestoreBlogStore {
	return &firestoreBlogStore{blogs: client.Collection("blogs")}
}

func toBlog(doc *firestore.DocumentSnapshot) (Blog, error) {
	var blog Blog
	if err := doc.DataTo(&blog); err != nil {
		return Blog{}, err
	}
	blog.ID = doc.Ref.ID
	return blog, nil
}

func (s *firestoreBlogStore) Get(ctx context.Context, id string) (Blog, error) {
	doc, err := s.blogs.Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return Blog{}, ErrNotFound
	}
	if err != nil {
		return Blog{}, err
	}
	return toBlog(doc)
}

// List runs canRead's predicate as a Firestore OR query, so a caller never fetches a private post
// they aren't on. Sorting happens here because ordering a disjunction by createdAt would need a
// composite index for each of its branches.
func (s *firestoreBlogStore) List(ctx context.Context, uid string) ([]Blog, error) {
	docs, err := s.blogs.WhereEntity(firestore.OrFilter{
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
		blog, err := toBlog(doc)
		if err != nil {
			return nil, err
		}
		blogs = append(blogs, blog)
	}
	slices.SortFunc(blogs, func(a, b Blog) int { return b.CreatedAt.Compare(a.CreatedAt) })

	return blogs, nil
}

func (s *firestoreBlogStore) Create(ctx context.Context, blog Blog) (Blog, error) {
	now := time.Now().UTC()
	blog.CreatedAt = now
	blog.UpdatedAt = now

	ref := s.blogs.NewDoc()
	if _, err := ref.Set(ctx, blog); err != nil {
		return Blog{}, err
	}
	blog.ID = ref.ID
	return blog, nil
}

func (s *firestoreBlogStore) Update(ctx context.Context, blog Blog) (Blog, error) {
	blog.UpdatedAt = time.Now().UTC()

	if _, err := s.blogs.Doc(blog.ID).Set(ctx, blog); err != nil {
		return Blog{}, err
	}
	return blog, nil
}

func (s *firestoreBlogStore) Delete(ctx context.Context, id string) error {
	_, err := s.blogs.Doc(id).Delete(ctx)
	return err
}

type firestoreUserStore struct {
	users *firestore.CollectionRef
}

func newFirestoreUserStore(client *firestore.Client) *firestoreUserStore {
	return &firestoreUserStore{users: client.Collection("users")}
}

func (s *firestoreUserStore) Get(ctx context.Context, id string) (User, error) {
	doc, err := s.users.Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}

	var user User
	if err := doc.DataTo(&user); err != nil {
		return User{}, err
	}
	user.ID = doc.Ref.ID
	return user, nil
}

func (s *firestoreUserStore) Put(ctx context.Context, user User) (User, error) {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	if _, err := s.users.Doc(user.ID).Set(ctx, user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *firestoreUserStore) Delete(ctx context.Context, id string) error {
	_, err := s.users.Doc(id).Delete(ctx)
	return err
}
