package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type userHandler struct {
	store UserStore
}

func newUserHandler(store UserStore) *userHandler {
	return &userHandler{store: store}
}

// decodeUser reads and validates a profile from the request body, writing the error response and
// returning false if it's malformed.
func decodeUser(w http.ResponseWriter, r *http.Request) (User, bool) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return User{}, false
	}
	if strings.TrimSpace(user.DisplayName) == "" {
		writeError(w, http.StatusBadRequest, "displayName is required")
		return User{}, false
	}
	return user, true
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

// Get returns a profile. Any signed-in caller may read any profile.
func (h *userHandler) Get(w http.ResponseWriter, r *http.Request) {
	user, err := h.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// Put creates or replaces the caller's own profile, responding 201 the first time and 200
// thereafter. Only the owner may write it.
func (h *userHandler) Put(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSelf(w, r)
	if !ok {
		return
	}

	body, ok := decodeUser(w, r)
	if !ok {
		return
	}
	body.ID = id

	existing, err := h.store.Get(r.Context(), id)
	created := errors.Is(err, ErrNotFound)
	if err != nil && !created {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !created {
		// A client must not be able to backdate its profile, so createdAt comes from the stored
		// document; a new profile leaves it zero for the store to stamp.
		body.CreatedAt = existing.CreatedAt
	}

	saved, err := h.store.Put(r.Context(), body)
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

// Delete removes the caller's own profile. Only the owner may delete it.
func (h *userHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := requireSelf(w, r)
	if !ok {
		return
	}

	// Firestore deletes are idempotent, so a missing profile is looked up first to give the same
	// 404 a client gets from GET.
	if _, err := h.store.Get(r.Context(), id); errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.store.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
