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

// reactionDocument is the stored shape of one reader's reactions to one target; see blogDocument
// for why it is separate.
//
// CommentID is stored even though the document key already carries it, because a target has to be
// queryable: DeleteTarget removes a comment's reactions by asking for them, and a key prefix is
// not something Firestore will filter on.
type reactionDocument struct {
	BlogSlug  string    `firestore:"blogSlug"`
	CommentID string    `firestore:"commentId"`
	UID       string    `firestore:"uid"`
	Emojis    []string  `firestore:"emojis"`
	UpdatedAt time.Time `firestore:"updatedAt"`
}

func reactionToDocument(reaction entity.Reaction) reactionDocument {
	return reactionDocument{
		BlogSlug:  reaction.Target.BlogSlug,
		CommentID: reaction.Target.CommentID,
		UID:       reaction.UID,
		Emojis:    reaction.Emojis,
		UpdatedAt: reaction.UpdatedAt,
	}
}

// toEntity rebuilds a reaction from its stored fields. As with a post - and unlike a comment - the
// key is never read back: everything that identifies a reaction is in the body, and the key is
// what makes the pair unique rather than what records it.
func (d reactionDocument) toEntity() entity.Reaction {
	return entity.Reaction{
		Target:    entity.ReactionTarget{BlogSlug: d.BlogSlug, CommentID: d.CommentID},
		UID:       d.UID,
		Emojis:    d.Emojis,
		UpdatedAt: d.UpdatedAt,
	}
}

var _ repository.ReactionRepository = (*ReactionRepository)(nil)

// ReactionRepository stores reactions in a subcollection of the post they are on
// ("blogs/{slug}/reactions/{target}-{uid}"), whether they are on the post itself or on one of its
// comments. Keeping a comment's reactions beside the post's rather than beneath the comment is
// what makes a page one query: a reader opening a post wants every reaction on it, and reactions
// nested under each comment would cost a query per comment to find.
//
// The key pairs the target with the reader (see entity.Reaction.Key), so "this reader has these
// reactions to this thing" is unique by construction, and two readers reacting at once write
// different documents and never contend.
type ReactionRepository struct {
	client *fs.Client
	blogs  *fs.CollectionRef
}

// NewReactionRepository returns a repository.ReactionRepository backed by the "reactions"
// subcollection beneath each post. The client is kept because Add and Remove are
// read-modify-writes and so run in a transaction.
func NewReactionRepository(client *fs.Client) *ReactionRepository {
	return &ReactionRepository{client: client, blogs: client.Collection("blogs")}
}

// reactionsFor resolves the collection beneath one post, validating the slug first: Doc panics on
// an empty path, so an unchecked slug would take the process down rather than fail the request.
func (r *ReactionRepository) reactionsFor(blogSlug string) (*fs.CollectionRef, error) {
	var post entity.Blog
	if err := post.SetSlug(blogSlug); err != nil {
		return nil, err
	}
	return r.blogs.Doc(post.Slug).Collection("reactions"), nil
}

func (r *ReactionRepository) List(ctx context.Context, blogSlug string) ([]entity.Reaction, error) {
	collection, err := r.reactionsFor(blogSlug)
	if err != nil {
		return nil, err
	}

	docs, err := collection.Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	reactions := make([]entity.Reaction, 0, len(docs))
	for _, doc := range docs {
		var stored reactionDocument
		if err := doc.DataTo(&stored); err != nil {
			return nil, err
		}
		reactions = append(reactions, stored.toEntity())
	}
	return reactions, nil
}

func (r *ReactionRepository) Add(ctx context.Context, target entity.ReactionTarget, uid, emoji string) (entity.Reaction, error) {
	return r.apply(ctx, target, uid, func(reaction *entity.Reaction) (bool, error) {
		return reaction.Add(emoji)
	})
}

func (r *ReactionRepository) Remove(ctx context.Context, target entity.ReactionTarget, uid, emoji string) (entity.Reaction, error) {
	return r.apply(ctx, target, uid, func(reaction *entity.Reaction) (bool, error) {
		return reaction.Remove(emoji), nil
	})
}

// apply is the read-modify-write both Add and Remove are. It runs in a transaction because a
// reader with two tabs open is the one way two writes can meet on this document - every other
// reader has a key of their own - and it writes nothing at all when the change would leave the
// record as it found it, so a repeated click costs no write.
func (r *ReactionRepository) apply(
	ctx context.Context,
	target entity.ReactionTarget,
	uid string,
	change func(*entity.Reaction) (bool, error),
) (entity.Reaction, error) {
	// Validated before the transaction so a bad target or emoji costs no round trip, and so Doc is
	// never handed a path it would panic on.
	if err := (entity.Reaction{Target: target, UID: uid}).Validate(); err != nil {
		return entity.Reaction{}, err
	}
	collection, err := r.reactionsFor(target.BlogSlug)
	if err != nil {
		return entity.Reaction{}, err
	}

	now := time.Now().UTC()
	doc := collection.Doc(entity.Reaction{Target: target, UID: uid}.Key())

	var reaction entity.Reaction
	err = r.client.RunTransaction(ctx, func(ctx context.Context, tx *fs.Transaction) error {
		// Rebuilt on every attempt rather than carried across them: a transaction is re-run when
		// it loses a race, and a second attempt has to start from what is stored now.
		reaction = entity.Reaction{Target: target, UID: uid}

		stored, err := tx.Get(doc)
		switch {
		case status.Code(err) == codes.NotFound:
			// The reader's first reaction to this target.
		case err != nil:
			return err
		default:
			var current reactionDocument
			if err := stored.DataTo(&current); err != nil {
				return err
			}
			reaction = current.toEntity()
		}

		changed, err := change(&reaction)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		reaction.UpdatedAt = now

		// A reader with nothing left is erased rather than stored as an empty row, so the
		// collection holds only reactions that are actually shown.
		if reaction.IsEmpty() {
			return tx.Delete(doc)
		}
		return tx.Set(doc, reactionToDocument(reaction))
	})
	if err != nil {
		return entity.Reaction{}, err
	}
	return reaction, nil
}

// DeleteTarget removes every reader's reactions to one target. It is a query rather than a key
// scan because the readers are not known here, and Firestore filters on fields rather than on the
// shape of a key - which is why the target is stored in the body as well as named by the key.
func (r *ReactionRepository) DeleteTarget(ctx context.Context, target entity.ReactionTarget) error {
	if err := target.Validate(); err != nil {
		return err
	}
	collection, err := r.reactionsFor(target.BlogSlug)
	if err != nil {
		return err
	}

	docs, err := collection.Where("commentId", "==", target.CommentID).Documents(ctx).GetAll()
	if err != nil {
		return err
	}

	// Deleted in one batch: a comment being moderated away should not leave half its reactions
	// behind because the request was cut off partway down the list.
	batch := r.client.BulkWriter(ctx)
	for _, doc := range docs {
		if _, err := batch.Delete(doc.Ref); err != nil {
			return err
		}
	}
	batch.End()

	return nil
}
