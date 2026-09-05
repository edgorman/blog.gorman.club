package firestore

import (
	"context"
	"strings"
	"time"

	fs "cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// userDocument is the stored shape of a profile; see blogDocument for why it is separate.
type userDocument struct {
	Username string `firestore:"username"`
	Bio      string `firestore:"bio"`
	// SubscribedUntil is omitted rather than stored as the zero time for an account that never
	// subscribed, so "has never paid" is an absent field rather than a date in 1 AD - which a
	// query for live subscriptions would otherwise have to know to exclude.
	SubscribedUntil *time.Time `firestore:"subscribedUntil,omitempty"`
	// StripeCustomerID is the payment provider's id for this account, and is absent for one that
	// has never reached a checkout. Like the expiry above it is written by SetSubscription alone.
	StripeCustomerID string    `firestore:"stripeCustomerId,omitempty"`
	CreatedAt        time.Time `firestore:"createdAt"`
	UpdatedAt        time.Time `firestore:"updatedAt"`
}

// The stored names of the two fields SetSubscription writes. They are named here rather than
// spelled into the update below because a Firestore field path is a string either way, and a typo
// in one would silently write a second field instead of the one meant.
const (
	subscribedUntilField  = "subscribedUntil"
	stripeCustomerIDField = "stripeCustomerId"
	updatedAtField        = "updatedAt"
)

// usernameDocument is a claim on one username, keyed by entity.User.UsernameKey. Uniqueness is a
// property of that key rather than something the code checks: a second claim on a name would have
// to be a second document at the same path, which Firestore will not hold.
type usernameDocument struct {
	UserID string `firestore:"userId"`
}

func userToDocument(user entity.User) userDocument {
	return userDocument{
		Username:         user.Username,
		Bio:              user.Bio,
		SubscribedUntil:  user.SubscribedUntil,
		StripeCustomerID: user.StripeCustomerID,
		CreatedAt:        user.CreatedAt,
		UpdatedAt:        user.UpdatedAt,
	}
}

// toEntity rebuilds a profile from its stored fields; id is the document key.
func (d userDocument) toEntity(id string) entity.User {
	return entity.User{
		ID:               id,
		Username:         d.Username,
		Bio:              d.Bio,
		SubscribedUntil:  d.SubscribedUntil,
		StripeCustomerID: d.StripeCustomerID,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
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
	client    *fs.Client
	users     *fs.CollectionRef
	usernames *fs.CollectionRef
}

// NewUserRepository returns a repository.UserRepository backed by the "users" collection, with the
// usernames its profiles hold reserved in a collection of their own. The client is kept because
// writes span both collections and so have to run in a transaction.
func NewUserRepository(client *fs.Client) *UserRepository {
	return &UserRepository{
		client:    client,
		users:     client.Collection("users"),
		usernames: client.Collection("usernames"),
	}
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

// GetByUsername resolves a name through its reservation rather than by querying the profiles for a
// matching field: both hops are lookups by document key, so neither needs an index and neither
// grows more expensive as the number of users does.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (entity.User, error) {
	key := strings.ToLower(strings.TrimSpace(username))
	// An empty key is not a document path Firestore can be asked about at all.
	if key == "" {
		return entity.User{}, repository.ErrNotFound
	}

	claim, err := r.usernames.Doc(key).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return entity.User{}, repository.ErrNotFound
	}
	if err != nil {
		return entity.User{}, err
	}

	var held usernameDocument
	if err := claim.DataTo(&held); err != nil {
		return entity.User{}, err
	}
	return r.Get(ctx, held.UserID)
}

// Put writes the profile and, when the username is new to it, moves the reservation across in the
// same transaction, so the two can never disagree about who holds what.
func (r *UserRepository) Put(ctx context.Context, user entity.User) (entity.User, error) {
	user, err := user.Normalized()
	if err != nil {
		return entity.User{}, err
	}

	now := time.Now().UTC()
	key := user.UsernameKey()

	err = r.client.RunTransaction(ctx, func(ctx context.Context, tx *fs.Transaction) error {
		// Firestore rejects a read that follows a write within a transaction, so everything the
		// decision below needs is fetched before anything is written.
		var current userDocument
		stored, err := tx.Get(r.users.Doc(user.ID))
		switch {
		case status.Code(err) == codes.NotFound:
			// A profile being written for the first time holds no username yet.
		case err != nil:
			return err
		default:
			if err := stored.DataTo(&current); err != nil {
				return err
			}
		}

		previous := strings.ToLower(current.Username)
		claiming := key != previous
		if claiming {
			// Reading the reservation here is what closes the race two callers picking the same
			// name would otherwise open: a transaction is serialized against the documents it read
			// by reference, so the second one to commit re-runs and finds the name gone.
			claim, err := tx.Get(r.usernames.Doc(key))
			switch {
			case status.Code(err) == codes.NotFound:
			case err != nil:
				return err
			default:
				var held usernameDocument
				if err := claim.DataTo(&held); err != nil {
					return err
				}
				if held.UserID != user.ID {
					return repository.ErrUsernameTaken
				}
			}
		}

		// createdAt comes from the stored profile rather than from the argument, so it cannot be
		// backdated by a caller that assembles one itself. The subscription comes from there for a
		// sharper version of the same reason: this is the write a caller's own request reaches, so
		// paid access surviving it is what stops an account granting itself a subscription by
		// sending one - and, incidentally, what stops a profile edit racing a webhook and undoing
		// a payment that landed between the read above and this write.
		user.SubscribedUntil = current.SubscribedUntil
		user.StripeCustomerID = current.StripeCustomerID
		user.CreatedAt = current.CreatedAt
		if user.CreatedAt.IsZero() {
			user.CreatedAt = now
		}
		user.UpdatedAt = now

		if claiming {
			if err := tx.Set(r.usernames.Doc(key), usernameDocument{UserID: user.ID}); err != nil {
				return err
			}
			if previous != "" {
				if err := tx.Delete(r.usernames.Doc(previous)); err != nil {
					return err
				}
			}
		}
		return tx.Set(r.users.Doc(user.ID), userToDocument(user))
	})
	if err != nil {
		return entity.User{}, err
	}
	return user, nil
}

// SetSubscription writes the account's paid access and nothing else, as an update of two fields
// rather than a read-modify-write of the whole profile: it is reached from a webhook, which runs
// concurrently with whatever the account itself is doing, and a profile edit in flight must not be
// overwritten by a payment landing - nor the other way round, which is what Put preserving these
// fields settles from the far side.
//
// A subscription that has ended clears the expiry rather than storing a past date, so "not paying"
// is one state in the datastore however it was arrived at (see entity.Subscription).
func (r *UserRepository) SetSubscription(ctx context.Context, id string, subscription entity.Subscription) error {
	updates := []fs.Update{
		{Path: subscribedUntilField, Value: subscription.SubscribedUntil()},
		{Path: updatedAtField, Value: time.Now().UTC()},
	}
	// An event that names no customer leaves the stored one alone rather than clearing it: the id
	// is how a later billing flow finds the customer this profile already has, and forgetting it
	// would mean making a second one for an account that is still paying through the first.
	if subscription.CustomerID != "" {
		updates = append(updates, fs.Update{Path: stripeCustomerIDField, Value: subscription.CustomerID})
	}

	// Update, rather than Set, so a webhook for an account with no profile fails loudly instead of
	// creating a document with a subscription and no owner - which no lookup could reach and no
	// username claim would match.
	_, err := r.users.Doc(id).Update(ctx, updates)
	if status.Code(err) == codes.NotFound {
		return repository.ErrNotFound
	}
	return err
}

// Delete removes the profile and the reservation together. Dropping only the profile would leave
// its username claimed by an account that no longer exists, and so unusable by anyone.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	return r.client.RunTransaction(ctx, func(ctx context.Context, tx *fs.Transaction) error {
		stored, err := tx.Get(r.users.Doc(id))
		if status.Code(err) == codes.NotFound {
			// Deleting an absent profile stays the no-op it was before reservations existed; the
			// service looks the profile up first when it wants to report a 404.
			return nil
		}
		if err != nil {
			return err
		}

		var current userDocument
		if err := stored.DataTo(&current); err != nil {
			return err
		}

		if key := strings.ToLower(current.Username); key != "" {
			if err := tx.Delete(r.usernames.Doc(key)); err != nil {
				return err
			}
		}
		return tx.Delete(r.users.Doc(id))
	})
}
