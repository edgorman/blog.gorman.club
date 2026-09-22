package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	blogv1 "github.com/edgorman/blog.gorman.club/services/backend/internal/gen/blog/v1"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// usernameAttempts bounds how many names a profile without one draws before giving up. Each draw
// comes from a pool of hundreds of thousands, so even a second attempt is unlikely and exhausting
// all of them is not something a real sign-up will reach.
const usernameAttempts = 5

// applyUpdate validates every field of an update through the entity's setters before touching
// user. An omitted username leaves whatever the profile already holds in place, so a client
// editing only its bio keeps the name it was given at sign-up without having to echo it back -
// which is what the request message's `optional username` buys and a bare string could not.
func applyUpdate(req *blogv1.UpdateCurrentUserRequest, user *entity.User) error {
	candidate := *user
	if req.Username != nil {
		if err := candidate.SetUsername(req.GetUsername()); err != nil {
			return err
		}
	}
	if err := candidate.SetBio(req.GetBio()); err != nil {
		return err
	}

	*user = candidate
	return nil
}

// userMessage is the public profile as anybody may read it. SubscribedUntil is absent from
// blogv1.User entirely rather than skipped here, so a lookup cannot disclose it by oversight.
func userMessage(user entity.User) *blogv1.User {
	return &blogv1.User{
		Id:        user.ID,
		Username:  user.Username,
		Bio:       user.Bio,
		CreatedAt: timestamppb.New(user.CreatedAt),
		UpdatedAt: timestamppb.New(user.UpdatedAt),
	}
}

// currentUser pairs a profile with what the account behind it may do. The capability is asked of
// the entitlement rather than computed here, so a client is told exactly what the chat routes
// would enforce (see entity.AssistantEntitlement). It needs nothing but the profile: the
// subscription is on it, and the account it belongs to is its own id.
//
// The capability rides on /users/me rather than on a route of its own because it is a property of
// the caller a client has just identified, and because it belongs nowhere else: a public profile
// must not disclose who has the assistant. SubscribedUntil travels the same way - an account may
// see when its own paid access runs out, and nobody else's lookup ever carries it.
func (s *Service) currentUser(user entity.User) *blogv1.CurrentUser {
	message := &blogv1.CurrentUser{
		Id:               user.ID,
		Username:         user.Username,
		Bio:              user.Bio,
		CreatedAt:        timestamppb.New(user.CreatedAt),
		UpdatedAt:        timestamppb.New(user.UpdatedAt),
		AssistantEnabled: s.cfg.AssistantEntitlement.Permission(entity.ActionUpdate, user).Allows(user.ID),
	}
	// Left nil for an account that has never subscribed, so the field stays absent from the body
	// rather than arriving as null - which is what it does today and what EmitDefaultValues
	// preserves (see writeProto).
	if user.SubscribedUntil != nil {
		message.SubscribedUntil = timestamppb.New(*user.SubscribedUntil)
	}
	return message
}

// GetCurrentUser returns the caller's own profile. It exists because a client holds a credential,
// not a username: this is how it discovers the name it was given at sign-up, and the only route
// that addresses a profile by the caller's uid rather than by a username.
func (s *Service) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.users.Get(r.Context(), uidFromContext(r.Context()))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeProto(w, http.StatusOK, s.currentUser(user))
}

// GetUser returns the profile holding a username. Any caller, signed in or not, may read any
// profile. The username is the only public handle a profile has: the Google `sub` it is keyed by
// never appears in a URL, so it cannot be used to address one.
//
// A 404 from here is also the answer to "is this name free?", so no separate availability endpoint
// is needed; profiles are public either way, so it discloses nothing a lookup would not.
func (s *Service) GetUser(w http.ResponseWriter, r *http.Request) {
	// Validating first answers a malformed name with the rule it broke, rather than with the 404
	// that looking it up would produce for it.
	var candidate entity.User
	if err := candidate.SetUsername(r.PathValue("username")); err != nil {
		writeValidationError(w, err)
		return
	}

	user, err := s.users.GetByUsername(r.Context(), candidate.Username)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeProto(w, http.StatusOK, userMessage(user))
}

// saveUser writes the profile, naming it first when it has none - which is every profile at
// sign-up, since clients are not asked to choose. Only the write can tell whether a name is free,
// so a collision is answered by drawing another rather than by checking beforehand, which would be
// slower and still racy.
func (s *Service) saveUser(ctx context.Context, user entity.User) (entity.User, error) {
	if user.Username != "" {
		return s.users.Put(ctx, user)
	}

	var err error
	for range usernameAttempts {
		user.Username = entity.NewUsername()

		var saved entity.User
		if saved, err = s.users.Put(ctx, user); !errors.Is(err, repository.ErrUsernameTaken) {
			return saved, err
		}
	}
	// Deliberately not wrapped: running out of draws is the server failing to name a profile, not
	// the caller asking for a name somebody else holds, and a client that sent no username has
	// nothing to do with a conflict.
	return entity.User{}, fmt.Errorf("no free username after %d attempts: %v", usernameAttempts, err)
}

// PutUser creates or replaces the caller's own profile, responding 201 the first time and 200
// thereafter. It is addressed as /users/me and resolves the owner from the verified credential, so
// a caller cannot name a profile other than its own to write - there is no longer an id to forge. Applying the request to the stored profile is what keeps createdAt un-backdatable,
// and what carries an existing username through a request that does not mention one.
func (s *Service) PutUser(w http.ResponseWriter, r *http.Request) {
	id := uidFromContext(r.Context())

	var body blogv1.UpdateCurrentUserRequest
	if err := readProto(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.users.Get(r.Context(), id)
	created := errors.Is(err, repository.ErrNotFound)
	if err != nil && !created {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	user.ID = id

	if err := applyUpdate(&body, &user); err != nil {
		writeValidationError(w, err)
		return
	}

	saved, err := s.saveUser(r.Context(), user)
	if errors.Is(err, repository.ErrUsernameTaken) {
		writeError(w, http.StatusConflict, "username already taken")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	// The same shape GetCurrentUser answers with, so a client that has just created its profile
	// learns what it may do without a second request.
	writeProto(w, status, s.currentUser(saved))
}

// DeleteUser removes the caller's own profile, addressed as /users/me for the same reason PutUser
// is: the owner comes from the credential, so only the owner can ever be the target.
func (s *Service) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := uidFromContext(r.Context())

	// Firestore deletes are idempotent, so a missing profile is looked up first to give the same
	// 404 a client gets from GET.
	if _, err := s.users.Get(r.Context(), id); errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := s.users.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
