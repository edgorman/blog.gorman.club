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

// maxListedComments is how many comments a thread hands back. A post is read far more often than
// it is commented on, so the bound is here rather than on the write: a busy thread is truncated to
// its newest replies instead of making every reader of the post pay for all of them. Nothing is
// deleted by it - the comments beyond it are still stored, and the cap is well above what a post
// on this blog will collect.
const maxListedComments = 500

// commentDocument is the stored shape of a comment; see blogDocument for why it is separate.
//
// BlogSlug is stored even though the document already sits beneath that post, so a comment read
// back carries everything that identifies it in its body - the same rule blogDocument and
// chatDocument follow - and so the entity is never assembled half from a path.
type commentDocument struct {
	BlogSlug  string    `firestore:"blogSlug"`
	AuthorID  string    `firestore:"authorId"`
	Body      string    `firestore:"body"`
	CreatedAt time.Time `firestore:"createdAt"`
}

func commentToDocument(comment entity.Comment) commentDocument {
	return commentDocument{
		BlogSlug:  comment.BlogSlug,
		AuthorID:  comment.AuthorID,
		Body:      comment.Body,
		CreatedAt: comment.CreatedAt,
	}
}

// toEntity rebuilds a comment from its stored fields and the key it was found at. The id is the
// one thing that does come from the key: unlike a post, a comment has no name of its own to store,
// so the document it lives at is the only place its id exists.
func documentToComment(doc *fs.DocumentSnapshot) (entity.Comment, error) {
	var stored commentDocument
	if err := doc.DataTo(&stored); err != nil {
		return entity.Comment{}, err
	}
	return entity.Comment{
		ID:        doc.Ref.ID,
		BlogSlug:  stored.BlogSlug,
		AuthorID:  stored.AuthorID,
		Body:      stored.Body,
		CreatedAt: stored.CreatedAt,
	}, nil
}

var _ repository.CommentRepository = (*CommentRepository)(nil)

// CommentRepository stores each comment in a subcollection of the post it is on
// ("blogs/{slug}/comments/{id}"), rather than in a collection of its own keyed by a slug field.
// That is what a comment is: it has no identity apart from its post, and no route names one that a
// /blogs/{slug} route would not have resolved first. It also keeps the thread a single-collection
// query - ordering by createdAt within one post needs no composite index, where the equivalent
// "where blogSlug == ... order by createdAt" over a flat collection would.
//
// The post's slug is what names the parent document, so entity.Blog.SetSlug is again what keeps
// the path parsable, and entity.Comment.SetID does the same for the leaf. Every method here
// refuses an empty or malformed pair rather than asking Firestore about a path it cannot parse.
type CommentRepository struct {
	blogs *fs.CollectionRef
}

// NewCommentRepository returns a repository.CommentRepository backed by the "comments"
// subcollection beneath each post. It holds the "blogs" collection rather than a collection of its
// own, since that is where the threads live.
func NewCommentRepository(client *fs.Client) *CommentRepository {
	return &CommentRepository{blogs: client.Collection("blogs")}
}

// commentsFor resolves the thread beneath one post, validating the slug first: Doc panics on an
// empty path, so an unchecked slug would take the process down rather than fail the request.
func (r *CommentRepository) commentsFor(blogSlug string) (*fs.CollectionRef, error) {
	var post entity.Blog
	if err := post.SetSlug(blogSlug); err != nil {
		return nil, err
	}
	return r.blogs.Doc(post.Slug).Collection("comments"), nil
}

// List returns the newest maxListedComments comments, handed back oldest first. The query orders
// them the other way round so that a truncated thread keeps its most recent replies rather than
// its first ones; the slice is reversed here, where it costs nothing, instead of by a second query.
func (r *CommentRepository) List(ctx context.Context, blogSlug string) ([]entity.Comment, error) {
	thread, err := r.commentsFor(blogSlug)
	if err != nil {
		return nil, err
	}

	docs, err := thread.OrderBy("createdAt", fs.Desc).Limit(maxListedComments).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	comments := make([]entity.Comment, 0, len(docs))
	for _, doc := range docs {
		comment, err := documentToComment(doc)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	slices.Reverse(comments)

	return comments, nil
}

// Get resolves one comment beneath one post. Both halves of the address are validated before
// either is turned into a path, and a bad half of either is ErrNotFound rather than a validation
// error: an id that cannot exist names nothing, exactly as a slug that cannot exist does.
func (r *CommentRepository) Get(ctx context.Context, blogSlug, id string) (entity.Comment, error) {
	var candidate entity.Comment
	if err := candidate.SetID(id); err != nil {
		return entity.Comment{}, repository.ErrNotFound
	}
	thread, err := r.commentsFor(blogSlug)
	if err != nil {
		return entity.Comment{}, repository.ErrNotFound
	}

	doc, err := thread.Doc(candidate.ID).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return entity.Comment{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.Comment{}, err
	}
	return documentToComment(doc)
}

// Create writes the comment at an id Firestore assigns, rather than at one derived from the
// comment. A post's slug comes from its title and a profile's key from its account, but a comment
// has nothing of its own to be named after - and nothing addresses one except a link built from
// the thread it was just read in, so an id nobody can guess costs nothing.
func (r *CommentRepository) Create(ctx context.Context, comment entity.Comment) (entity.Comment, error) {
	// Validate is what guarantees the parent path is addressable, so it has to come before any
	// Doc call; commentsFor re-checks the same slug, which is what makes that safe in either order.
	if err := comment.Validate(); err != nil {
		return entity.Comment{}, err
	}
	thread, err := r.commentsFor(comment.BlogSlug)
	if err != nil {
		return entity.Comment{}, err
	}

	comment.CreatedAt = time.Now().UTC()

	doc := thread.NewDoc()
	comment.ID = doc.ID
	if _, err := doc.Create(ctx, commentToDocument(comment)); err != nil {
		return entity.Comment{}, err
	}
	return comment, nil
}

// Delete erases the comment. Firestore deletes are idempotent, so removing one that is already
// gone succeeds - the service reads it first anyway, since who may delete it is a property of the
// comment (see entity.Comment.Permission).
func (r *CommentRepository) Delete(ctx context.Context, blogSlug, id string) error {
	var candidate entity.Comment
	if err := candidate.SetID(id); err != nil {
		return repository.ErrNotFound
	}
	thread, err := r.commentsFor(blogSlug)
	if err != nil {
		return repository.ErrNotFound
	}

	_, err = thread.Doc(candidate.ID).Delete(ctx)
	return err
}
