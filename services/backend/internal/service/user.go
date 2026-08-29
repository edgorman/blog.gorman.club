package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// usernameAttempts bounds how many names a profile without one draws before giving up. Each draw
// comes from a pool of hundreds of thousands, so even a second attempt is unlikely and exhausting
// all of them is not something a real sign-up will reach.
const usernameAttempts = 5

// userRequest is the client-settable half of a profile; the id comes from the path and the
// timestamps from the server.
type userRequest struct {
	// A pointer so an omitted username is distinguishable from one explicitly set to "": the first
	// keeps the name the profile holds, the second is a name SetUsername rejects. A plain string
	// would conflate them and answer a cleared field with a silent success.
	Username *string `json:"username"`
	Bio      string  `json:"bio"`
}

// applyTo validates every field through the entity's setters before touching user. An omitted
// username leaves whatever the profile already holds in place, so a client editing only its bio
// keeps the name it was given at sign-up without having to echo it back.
func (u userRequest) applyTo(user *entity.User) error {
	candidate := *user
	if u.Username != nil {
		if err := candidate.SetUsername(*u.Username); err != nil {
			return err
		}
	}
	if err := candidate.SetBio(u.Bio); err != nil {
		return err
	}

	*user = candidate
	return nil
}

// currentUserResponse is the caller's own profile, plus what this deployment lets that account do.
//
// The capability rides on /users/me rather than on a route of its own because it is a property of
// the caller a client has just identified, and because it belongs nowhere else: a public profile
// must not disclose who has the assistant, so it cannot go on entity.User itself. A client uses it
// to decide whether to offer the assistant at all - the routes enforce it either way, this only
// keeps a button off the screen for somebody who would be told no.
type currentUserResponse struct {
	entity.User
	AssistantEnabled bool `json:"assistantEnabled"`
}

// currentUser pairs a profile with what the credential behind it may do. The capability comes from
// the caller rather than from the profile, since that is what the allowlist is keyed on.
func (s *Service) currentUser(ctx context.Context, user entity.User) currentUserResponse {
	return currentUserResponse{
		User:             user,
		AssistantEnabled: s.cfg.AssistantAllowlist.Allows(callerFromContext(ctx)),
	}
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

	writeJSON(w, http.StatusOK, s.currentUser(r.Context(), user))
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

	writeJSON(w, http.StatusOK, user)
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

	var body userRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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

	if err := body.applyTo(&user); err != nil {
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
	writeJSON(w, status, s.currentUser(r.Context(), saved))
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
