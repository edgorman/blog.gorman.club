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

// blogKeySeparator joins an owner and a slug into the document key below. A slug cannot contain
// it (entity.Blog.SetSlug admits only lowercase letters, digits, and hyphens), so no two
// owner/slug pairs can produce the same key.
const blogKeySeparator = "_"

// blogKey is the document key a post is stored at. Scoping the key to the owner is the whole of
// how slugs are made unique per author rather than globally: uniqueness is a property of the key
// (Create refuses to overwrite one), so two authors can both hold "hello-world" while one author
// cannot hold it twice.
//
// The key is derived rather than stored: ownerId and slug are the document's own fields, and this
// is a function of them, so the two can never disagree about where a post lives.
func blogKey(ownerID, slug string) string {
	return ownerID + blogKeySeparator + slug
}

// blogDocument is the stored shape of a blog. It exists so Firestore's field names live here
// rather than as a second set of tags on entity.Blog.
type blogDocument struct {
	OwnerID        string    `firestore:"ownerId"`
	Slug           string    `firestore:"slug"`
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
		Slug:           blog.Slug,
		Title:          blog.Title,
		Content:        blog.Content,
		Visibility:     string(blog.Visibility),
		AllowedUserIDs: blog.AllowedUserIDs,
		CreatedAt:      blog.CreatedAt,
		UpdatedAt:      blog.UpdatedAt,
	}
}

// toEntity rebuilds a blog from its stored fields. Unlike a user - whose id is its document key -
// a post carries everything that identifies it in the body, so the key is never read back.
func (d blogDocument) toEntity() entity.Blog {
	return entity.Blog{
		Slug:           d.Slug,
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
	return stored.toEntity(), nil
}

var _ repository.BlogRepository = (*BlogRepository)(nil)

type BlogRepository struct {
	blogs *fs.CollectionRef
}

// NewBlogRepository returns a repository.BlogRepository backed by the "blogs" collection.
func NewBlogRepository(client *fs.Client) *BlogRepository {
	return &BlogRepository{blogs: client.Collection("blogs")}
}

// Get resolves a post by the pair that names it. Both halves are required: an empty one would ask
// Firestore about a document path it cannot parse.
func (r *BlogRepository) Get(ctx context.Context, ownerID, slug string) (entity.Blog, error) {
	if ownerID == "" || slug == "" {
		return entity.Blog{}, repository.ErrNotFound
	}

	doc, err := r.blogs.Doc(blogKey(ownerID, slug)).Get(ctx)
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

// Create writes the post at the key its owner and slug name, rather than at one Firestore picks.
// Uniqueness is therefore a property of that key rather than something checked here: Create
// (unlike Set) refuses to overwrite an existing document, so two of an author's posts racing for
// the same title cannot both take it, and the loser is told to draw a suffixed slug instead.
func (r *BlogRepository) Create(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	// Validate is what guarantees both halves of the key are present and addressable; Doc panics
	// on an empty path, so this has to come first.
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}

	now := time.Now().UTC()
	blog.CreatedAt = now
	blog.UpdatedAt = now

	_, err := r.blogs.Doc(blogKey(blog.OwnerID, blog.Slug)).Create(ctx, blogToDocument(blog))
	if status.Code(err) == codes.AlreadyExists {
		return entity.Blog{}, repository.ErrSlugTaken
	}
	if err != nil {
		return entity.Blog{}, err
	}
	return blog, nil
}

// Update overwrites the post in place, slug included - which is to say the post never moves.
// Deriving a fresh slug from an edited title would break every link to the post and leave the old
// one free for another of the author's posts to take, so it stays as it was assigned at creation.
func (r *BlogRepository) Update(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}

	blog.UpdatedAt = time.Now().UTC()

	if _, err := r.blogs.Doc(blogKey(blog.OwnerID, blog.Slug)).Set(ctx, blogToDocument(blog)); err != nil {
		return entity.Blog{}, err
	}
	return blog, nil
}

func (r *BlogRepository) Delete(ctx context.Context, ownerID, slug string) error {
	if ownerID == "" || slug == "" {
		return repository.ErrNotFound
	}

	_, err := r.blogs.Doc(blogKey(ownerID, slug)).Delete(ctx)
	return err
}
