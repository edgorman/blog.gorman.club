package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

type userHandler struct {
	store UserStore
}

func newUserHandler(store UserStore) *userHandler {
	return &userHandler{store: store}
}

// Get returns a user's profile. Any signed-in caller may read any profile, matching
// firestore.rules' `allow read: if request.auth != null`.
func (h *userHandler) Get(w http.ResponseWriter, r *http.Request) {
	user, err := h.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// Put creates or replaces a user's own profile. Only the profile's owner may write it, matching
// firestore.rules' `allow write: if request.auth.uid == userId`.
func (h *userHandler) Put(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if uidFromContext(r.Context()) != id {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var body User
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	body.ID = id

	if err := h.store.Set(r.Context(), body); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, body)
}
