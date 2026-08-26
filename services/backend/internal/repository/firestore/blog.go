// Package firestore implements the repository interfaces against Cloud Firestore.
package firestore

import (
	"context"
	"slices"
	"time"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// blogDocument is the stored shape of a blog. It exists so Firestore's field names live here
// rather than as a second set of tags on entity.Blog, whose ID is the document key, not a field.
type blogDocument struct {
	OwnerID        string    `firestore:"ownerId"`
	Title          string    `firestore:"title"`
	Content        string    `firestore:"content"`
	Visibility     string    `firestore:"visibility"`
	AllowedUserIDs []string  `firestore:"allowedUserIds"`
	CreatedAt      time.Time `firestore:"createdAt"`
	UpdatedAt      time.Time `firestore:"updatedAt"`
}

func blogToDocument(blog entity.Blog) blogDocument {
	return blogDocument{
		OwnerID:        blog.OwnerID,
		Title:          blog.Title,
		Content:        blog.Content,
		Visibility:     string(blog.Visibility),
		AllowedUserIDs: blog.AllowedUserIDs,
		CreatedAt:      blog.CreatedAt,
		UpdatedAt:      blog.UpdatedAt,
	}
}

// toEntity rebuilds a blog from its stored fields; id is the document key, which Firestore keeps
// outside the document body.
func (d blogDocument) toEntity(id string) entity.Blog {
	return entity.Blog{
		ID:             id,
		OwnerID:        d.OwnerID,
		Title:          d.Title,
		Content:        d.Content,
		Visibility:     entity.Visibility(d.Visibility),
		AllowedUserIDs: d.AllowedUserIDs,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func documentToBlog(doc *fs.DocumentSnapshot) (entity.Blog, error) {
	var stored blogDocument
	if err := doc.DataTo(&stored); err != nil {
		return entity.Blog{}, err
	}
	return stored.toEntity(doc.Ref.ID), nil
}

var _ repository.BlogRepository = (*BlogRepository)(nil)

type BlogRepository struct {
	blogs *fs.CollectionRef
}

// NewBlogRepository returns a repository.BlogRepository backed by the "blogs" collection.
func NewBlogRepository(client *fs.Client) *BlogRepository {
	return &BlogRepository{blogs: client.Collection("blogs")}
}

func (r *BlogRepository) Get(ctx context.Context, id string) (entity.Blog, error) {
	doc, err := r.blogs.Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return entity.Blog{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.Blog{}, err
	}
	return documentToBlog(doc)
}

// List runs CanBeReadBy's predicate as a Firestore OR query, so a caller never fetches a private
// post they aren't on. Sorting happens here because ordering a disjunction by createdAt would need
// a composite index for each of its branches.
func (r *BlogRepository) List(ctx context.Context, uid string) ([]entity.Blog, error) {
	docs, err := r.blogs.WhereEntity(fs.OrFilter{
		Filters: []fs.EntityFilter{
			fs.PropertyFilter{Path: "visibility", Operator: "==", Value: string(entity.VisibilityPublic)},
			fs.PropertyFilter{Path: "ownerId", Operator: "==", Value: uid},
			fs.PropertyFilter{Path: "allowedUserIds", Operator: "array-contains", Value: uid},
		},
	}).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	blogs := make([]entity.Blog, 0, len(docs))
	for _, doc := range docs {
		blog, err := documentToBlog(doc)
		if err != nil {
			return nil, err
		}
		blogs = append(blogs, blog)
	}
	slices.SortFunc(blogs, func(a, b entity.Blog) int { return b.CreatedAt.Compare(a.CreatedAt) })

	return blogs, nil
}

func (r *BlogRepository) Create(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}

	now := time.Now().UTC()
	blog.CreatedAt = now
	blog.UpdatedAt = now

	ref := r.blogs.NewDoc()
	if _, err := ref.Set(ctx, blogToDocument(blog)); err != nil {
		return entity.Blog{}, err
	}
	blog.ID = ref.ID
	return blog, nil
}

func (r *BlogRepository) Update(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}

	blog.UpdatedAt = time.Now().UTC()

	if _, err := r.blogs.Doc(blog.ID).Set(ctx, blogToDocument(blog)); err != nil {
		return entity.Blog{}, err
	}
	return blog, nil
}

func (r *BlogRepository) Delete(ctx context.Context, id string) error {
	_, err := r.blogs.Doc(id).Delete(ctx)
	return err
}
