package main

import (
	"encoding/json"
	"errors"
	"net/http"
)

type blogHandler struct {
	store BlogStore
}

func newBlogHandler(store BlogStore) *blogHandler {
	return &blogHandler{store: store}
}

func isValidVisibility(v string) bool {
	return v == "public" || v == "private"
}

// List returns every blog visible to the caller (see Blog.visibleTo).
func (h *blogHandler) List(w http.ResponseWriter, r *http.Request) {
	blogs, err := h.store.List(r.Context(), uidFromContext(r.Context()))
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, blogs)
}

// Get returns a single blog if it's visible to the caller. A private blog the caller can't see
// responds identically to a missing one, so its existence isn't leaked.
func (h *blogHandler) Get(w http.ResponseWriter, r *http.Request) {
	blog, err := h.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !blog.visibleTo(uidFromContext(r.Context())) {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, blog)
}

// Create makes a new blog owned by the caller. ownerId is always taken from the verified caller,
// never from the request body.
func (h *blogHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body Blog
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidVisibility(body.Visibility) {
		http.Error(w, `visibility must be "public" or "private"`, http.StatusBadRequest)
		return
	}
	body.OwnerID = uidFromContext(r.Context())

	created, err := h.store.Create(r.Context(), body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// Update replaces a blog's fields. Only the owner may update it, matching firestore.rules'
// `allow update: if resource.data.ownerId == request.auth.uid`.
func (h *blogHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	callerUID := uidFromContext(r.Context())

	existing, err := h.store.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing.OwnerID != callerUID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var body Blog
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !isValidVisibility(body.Visibility) {
		http.Error(w, `visibility must be "public" or "private"`, http.StatusBadRequest)
		return
	}
	body.OwnerID = callerUID

	updated, err := h.store.Update(r.Context(), id, body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// Delete removes a blog. Only the owner may delete it, matching firestore.rules'
// `allow delete: if resource.data.ownerId == request.auth.uid`.
func (h *blogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	callerUID := uidFromContext(r.Context())

	existing, err := h.store.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "blog not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if existing.OwnerID != callerUID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.store.Delete(r.Context(), id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
