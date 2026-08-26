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

// userDocument is the stored shape of a profile; see blogDocument for why it is separate.
type userDocument struct {
	DisplayName string    `firestore:"displayName"`
	Bio         string    `firestore:"bio"`
	CreatedAt   time.Time `firestore:"createdAt"`
	UpdatedAt   time.Time `firestore:"updatedAt"`
}

func userToDocument(user entity.User) userDocument {
	return userDocument{
		DisplayName: user.DisplayName,
		Bio:         user.Bio,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

// toEntity rebuilds a profile from its stored fields; id is the document key.
func (d userDocument) toEntity(id string) entity.User {
	return entity.User{
		ID:          id,
		DisplayName: d.DisplayName,
		Bio:         d.Bio,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func documentToUser(doc *fs.DocumentSnapshot) (entity.User, error) {
	var stored userDocument
	if err := doc.DataTo(&stored); err != nil {
		return entity.User{}, err
	}
	return stored.toEntity(doc.Ref.ID), nil
}

var _ repository.UserRepository = (*UserRepository)(nil)

type UserRepository struct {
	users *fs.CollectionRef
}

// NewUserRepository returns a repository.UserRepository backed by the "users" collection.
func NewUserRepository(client *fs.Client) *UserRepository {
	return &UserRepository{users: client.Collection("users")}
}

func (r *UserRepository) Get(ctx context.Context, id string) (entity.User, error) {
	doc, err := r.users.Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return entity.User{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.User{}, err
	}
	return documentToUser(doc)
}

func (r *UserRepository) Put(ctx context.Context, user entity.User) (entity.User, error) {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	if _, err := r.users.Doc(user.ID).Set(ctx, userToDocument(user)); err != nil {
		return entity.User{}, err
	}
	return user, nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.users.Doc(id).Delete(ctx)
	return err
}
