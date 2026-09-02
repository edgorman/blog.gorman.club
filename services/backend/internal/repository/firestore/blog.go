// Package firestore implements the repository interfaces against Cloud Firestore.
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

// blogDocument is the stored shape of a blog. It exists so Firestore's field names live here
// rather than as a second set of tags on entity.Blog.
type blogDocument struct {
	OwnerID        string     `firestore:"ownerId"`
	Slug           string     `firestore:"slug"`
	Title          string     `firestore:"title"`
	Content        string     `firestore:"content"`
	Visibility     string     `firestore:"visibility"`
	AllowedUserIDs []string   `firestore:"allowedUserIds"`
	CreatedAt      time.Time  `firestore:"createdAt"`
	UpdatedAt      time.Time  `firestore:"updatedAt"`
	DeletedAt      *time.Time `firestore:"deletedAt"`
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
		DeletedAt:      blog.DeletedAt,
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
		DeletedAt:      d.DeletedAt,
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

// BlogRepository stores each post at its slug, unqualified, and that is the whole of how slugs are
// made unique across every author: uniqueness is a property of the document key (Create refuses to
// overwrite one), so no two posts hold "hello-world" whoever wrote them. That global uniqueness is
// what lets a post be addressed as /blogs/{slug}, with the owner kept as a field rather than as
// part of where the post lives.
//
// entity.Blog.SetSlug is what keeps a slug usable as a key: it admits only lowercase letters,
// digits, and single hyphens, which excludes every form Firestore refuses in a document path.
type BlogRepository struct {
	blogs *fs.CollectionRef
}

// NewBlogRepository returns a repository.BlogRepository backed by the "blogs" collection.
func NewBlogRepository(client *fs.Client) *BlogRepository {
	return &BlogRepository{blogs: client.Collection("blogs")}
}

// Get resolves a post by the slug that names it. A slug is required: an empty one would ask
// Firestore about a document path it cannot parse. A soft-deleted post answers the same as one
// that was never there: its document still exists, but Get is a read path and a deleted post has
// nothing left to read.
func (r *BlogRepository) Get(ctx context.Context, slug string) (entity.Blog, error) {
	if slug == "" {
		return entity.Blog{}, repository.ErrNotFound
	}

	doc, err := r.blogs.Doc(slug).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return entity.Blog{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.Blog{}, err
	}
	blog, err := documentToBlog(doc)
	if err != nil {
		return entity.Blog{}, err
	}
	if blog.IsDeleted() {
		return entity.Blog{}, repository.ErrNotFound
	}
	return blog, nil
}

// maxBlogListPageSize is the largest page List ever returns, whatever a caller asks for - so a
// param nobody validated cannot turn a page into the very full-collection read pagination exists
// to avoid.
const maxBlogListPageSize = 50

// listQuery is the unfiltered-by-page half of List: which documents are candidates at all.
//
// A profile feed (params.OwnerUID set) narrows to one author's documents with a plain equality
// filter; whether the caller may actually read each one is left to the CanBeReadBy check List
// applies to every document either query yields, rather than folded in here as a second Firestore
// filter. That keeps this to the one composite index (ownerId, createdAt) List's other branch
// already needs, at the cost of walking past a private post of that author's before reaching the
// next visible one - a cost bounded by that one author's post count, not the whole collection.
//
// The general feed (params.OwnerUID empty) runs CanBeReadBy's predicate as a Firestore OR query,
// so a caller never fetches a private post they aren't on in the first place. Both branches still
// recheck CanBeReadBy in List: that is what keeps this filter and entity.Blog.CanBeReadBy from
// drifting apart unnoticed, since a mismatch here would just cost an extra document read rather
// than leak one.
func (r *BlogRepository) listQuery(uid string, ownerUID string) fs.Query {
	if ownerUID != "" {
		return r.blogs.Where("ownerId", "==", ownerUID)
	}
	return r.blogs.WhereEntity(fs.OrFilter{
		Filters: []fs.EntityFilter{
			fs.PropertyFilter{Path: "visibility", Operator: "==", Value: string(entity.VisibilityPublic)},
			fs.PropertyFilter{Path: "ownerId", Operator: "==", Value: uid},
			fs.PropertyFilter{Path: "allowedUserIds", Operator: "array-contains", Value: uid},
		},
	})
}

// List walks listQuery's candidates in createdAt order, one Firestore page at a time, discarding
// whatever the caller may not read and whatever is soft-deleted as it goes - until it has
// params.Limit+1 live, readable posts or the candidates run out. The +1 is never returned; its
// presence is what hasMore reports, so a caller learns whether to offer a further page without
// this answering more than it was asked for. Ordering is a plain OrderBy here, unlike the old
// unpaginated List's in-memory sort, because a cursor has to be a position the query itself
// understands.
func (r *BlogRepository) List(ctx context.Context, uid string, params repository.ListParams) ([]entity.Blog, bool, error) {
	limit := params.Limit
	if limit <= 0 || limit > maxBlogListPageSize {
		limit = maxBlogListPageSize
	}

	query := r.listQuery(uid, params.OwnerUID).OrderBy("createdAt", fs.Desc)
	cursor := params.StartAfter

	blogs := make([]entity.Blog, 0, limit)
	for len(blogs) <= limit {
		page := query.Limit(limit + 1 - len(blogs))
		if !cursor.IsZero() {
			page = page.StartAfter(cursor)
		}

		docs, err := page.Documents(ctx).GetAll()
		if err != nil {
			return nil, false, err
		}
		if len(docs) == 0 {
			break
		}

		for _, doc := range docs {
			blog, err := documentToBlog(doc)
			if err != nil {
				return nil, false, err
			}
			// Advanced whether or not the post is kept, so the next round of this loop - or the next
			// caller's page - continues from the last candidate actually seen, not the last one kept.
			cursor = blog.CreatedAt
			if blog.IsDeleted() || !blog.CanBeReadBy(uid) {
				continue
			}
			blogs = append(blogs, blog)
		}
	}

	hasMore := len(blogs) > limit
	if hasMore {
		blogs = blogs[:limit]
	}
	return blogs, hasMore, nil
}

// Create writes the post at the key its slug names, rather than at one Firestore picks.
// Uniqueness is therefore a property of that key rather than something checked here: Create
// (unlike Set) refuses to overwrite an existing document, so two posts racing for the same title
// cannot both take it, and the loser is told to draw a suffixed slug instead.
func (r *BlogRepository) Create(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	// Validate is what guarantees the key is present and addressable; Doc panics on an empty path,
	// so this has to come first.
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}

	now := time.Now().UTC()
	blog.CreatedAt = now
	blog.UpdatedAt = now

	_, err := r.blogs.Doc(blog.Slug).Create(ctx, blogToDocument(blog))
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
// one free for another post to take, so it stays as it was assigned at creation.
func (r *BlogRepository) Update(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}

	blog.UpdatedAt = time.Now().UTC()

	if _, err := r.blogs.Doc(blog.Slug).Set(ctx, blogToDocument(blog)); err != nil {
		return entity.Blog{}, err
	}
	return blog, nil
}

// Delete soft-deletes a post by stamping DeletedAt rather than removing its document: the
// collection never loses a post, it is only marked gone. It is a single targeted field write
// rather than a read-modify-write, so it costs no more than the hard delete it replaces.
func (r *BlogRepository) Delete(ctx context.Context, slug string) error {
	if slug == "" {
		return repository.ErrNotFound
	}

	now := time.Now().UTC()
	_, err := r.blogs.Doc(slug).Update(ctx, []fs.Update{{Path: "deletedAt", Value: now}})
	if status.Code(err) == codes.NotFound {
		return repository.ErrNotFound
	}
	return err
}
