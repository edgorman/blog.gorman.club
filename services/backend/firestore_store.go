package main

import (
	"context"
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	usersCollection = "users"
	blogsCollection = "blogs"
)

type firestoreUserStore struct {
	client *firestore.Client
}

func newFirestoreUserStore(client *firestore.Client) *firestoreUserStore {
	return &firestoreUserStore{client: client}
}

func (s *firestoreUserStore) Get(ctx context.Context, id string) (User, error) {
	doc, err := s.client.Collection(usersCollection).Doc(id).Get(ctx)
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

func (s *firestoreUserStore) Set(ctx context.Context, user User) error {
	_, err := s.client.Collection(usersCollection).Doc(user.ID).Set(ctx, user)
	return err
}

type firestoreBlogStore struct {
	client *firestore.Client
}

func newFirestoreBlogStore(client *firestore.Client) *firestoreBlogStore {
	return &firestoreBlogStore{client: client}
}

func (s *firestoreBlogStore) collection() *firestore.CollectionRef {
	return s.client.Collection(blogsCollection)
}

func blogFromDoc(doc *firestore.DocumentSnapshot) (Blog, error) {
	var blog Blog
	if err := doc.DataTo(&blog); err != nil {
		return Blog{}, err
	}
	blog.ID = doc.Ref.ID
	return blog, nil
}

func (s *firestoreBlogStore) Get(ctx context.Context, id string) (Blog, error) {
	doc, err := s.collection().Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return Blog{}, ErrNotFound
	}
	if err != nil {
		return Blog{}, err
	}
	return blogFromDoc(doc)
}

// List mirrors firestore.rules' read condition for /blogs/{blogId}: public blogs, blogs the
// caller owns, and private blogs where callerUID is in allowedUserIds. Firestore has no single
// query that ORs across those three, so they're run separately and merged, deduplicating by ID.
func (s *firestoreBlogStore) List(ctx context.Context, callerUID string) ([]Blog, error) {
	seen := make(map[string]Blog)

	collect := func(iter *firestore.DocumentIterator) error {
		defer iter.Stop()
		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				return nil
			}
			if err != nil {
				return err
			}
			blog, err := blogFromDoc(doc)
			if err != nil {
				return err
			}
			seen[blog.ID] = blog
		}
	}

	if err := collect(s.collection().Where("visibility", "==", "public").Documents(ctx)); err != nil {
		return nil, err
	}

	if callerUID != "" {
		if err := collect(s.collection().Where("ownerId", "==", callerUID).Documents(ctx)); err != nil {
			return nil, err
		}
		if err := collect(s.collection().Where("allowedUserIds", "array-contains", callerUID).Documents(ctx)); err != nil {
			return nil, err
		}
	}

	blogs := make([]Blog, 0, len(seen))
	for _, blog := range seen {
		blogs = append(blogs, blog)
	}
	sort.Slice(blogs, func(i, j int) bool { return blogs[i].CreatedAt.After(blogs[j].CreatedAt) })
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

func (s *firestoreBlogStore) Update(ctx context.Context, id string, blog Blog) (Blog, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return Blog{}, err
	}
	blog.CreatedAt = existing.CreatedAt
	blog.UpdatedAt = time.Now().UTC()

	if _, err := s.collection().Doc(id).Set(ctx, blog); err != nil {
		return Blog{}, err
	}
	blog.ID = id
	return blog, nil
}

func (s *firestoreBlogStore) Delete(ctx context.Context, id string) error {
	_, err := s.collection().Doc(id).Delete(ctx)
	return err
}
