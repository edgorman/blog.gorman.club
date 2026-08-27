package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// userRequest is the client-settable half of a profile; the id comes from the path and the
// timestamps from the server.
type userRequest struct {
	DisplayName string `json:"displayName"`
	Bio         string `json:"bio"`
}

// applyTo validates every field through the entity's setters before touching user.
func (u userRequest) applyTo(user *entity.User) error {
	candidate := *user
	if err := candidate.SetDisplayName(u.DisplayName); err != nil {
		return err
	}
	if err := candidate.SetBio(u.Bio); err != nil {
		return err
	}

	*user = candidate
	return nil
}

// requireSelf checks the {id} path value is the caller's own uid, writing the error response and
// returning false otherwise.
func requireSelf(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if id != uidFromContext(r.Context()) {
		writeError(w, http.StatusForbidden, "forbidden")
		return "", false
	}
	return id, true
}

// GetUser returns a profile. Any caller, signed in or not, may read any profile.
func (s *Service) GetUser(w http.ResponseWriter, r *http.Request) {
	user, err := s.users.Get(r.Context(), r.PathValue("id"))
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

// PutUser creates or replaces the caller's own profile, responding 201 the first time and 200
// thereafter. Applying the request to the stored profile is what keeps createdAt un-backdatable.
func (s *Service) PutUser(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSelf(w, r)
	if !ok {
		return
	}

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

	saved, err := s.users.Put(r.Context(), user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, saved)
}

// DeleteUser removes the caller's own profile. Only the owner may delete it.
func (s *Service) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSelf(w, r)
	if !ok {
		return
	}

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
