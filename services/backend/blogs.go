package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
)

type blogHandler struct {
	store BlogStore
}

func newBlogHandler(store BlogStore) *blogHandler {
	return &blogHandler{store: store}
}

// decodeBlog reads and validates a blog from the request body, writing the error response and
// returning false if it's malformed.
func decodeBlog(w http.ResponseWriter, r *http.Request) (Blog, bool) {
	var blog Blog
	if err := json.NewDecoder(r.Body).Decode(&blog); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return Blog{}, false
	}
	if blog.Visibility != "public" && blog.Visibility != "private" {
		writeError(w, http.StatusBadRequest, `visibility must be "public" or "private"`)
		return Blog{}, false
	}
	return blog, true
}

// canRead reports whether uid may read blog: public posts are readable by any signed-in caller,
// private ones only by their owner or a whitelisted uid. This is the single definition of read
// access - firestoreBlogStore.List runs the same predicate as a Firestore query so that private
// posts are never fetched in the first place.
func canRead(blog Blog, uid string) bool {
	if blog.Visibility == "public" || blog.OwnerID == uid {
		return true
	}
	return slices.Contains(blog.AllowedUserIDs, uid)
}

// requireOwnedBlog loads the blog named by the {id} path value and checks the caller owns it,
// writing the error response and returning false otherwise. This service holds the only
// credentials for the collection, so it is the enforcement point for every write.
func (h *blogHandler) requireOwnedBlog(w http.ResponseWriter, r *http.Request) (Blog, bool) {
	blog, err := h.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "blog not found")
		return Blog{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return Blog{}, false
	}
	if blog.OwnerID != uidFromContext(r.Context()) {
		writeError(w, http.StatusForbidden, "forbidden")
		return Blog{}, false
	}
	return blog, true
}

// List returns every blog the caller is allowed to read, newest first.
func (h *blogHandler) List(w http.ResponseWriter, r *http.Request) {
	blogs, err := h.store.List(r.Context(), uidFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if blogs == nil {
		// An empty collection is an empty JSON array, never null.
		blogs = []Blog{}
	}

	writeJSON(w, http.StatusOK, blogs)
}

// Get returns a single blog, provided the caller is allowed to read it.
func (h *blogHandler) Get(w http.ResponseWriter, r *http.Request) {
	blog, err := h.store.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "blog not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !canRead(blog, uidFromContext(r.Context())) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, blog)
}

// Create makes a new blog owned by the caller. ownerId is always taken from the verified caller,
// never from the request body.
func (h *blogHandler) Create(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeBlog(w, r)
	if !ok {
		return
	}
	body.OwnerID = uidFromContext(r.Context())

	created, err := h.store.Create(r.Context(), body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// Update replaces a blog's fields. Only the owner may update it.
func (h *blogHandler) Update(w http.ResponseWriter, r *http.Request) {
	existing, ok := h.requireOwnedBlog(w, r)
	if !ok {
		return
	}

	body, ok := decodeBlog(w, r)
	if !ok {
		return
	}
	body.ID = existing.ID
	body.OwnerID = existing.OwnerID
	body.CreatedAt = existing.CreatedAt

	updated, err := h.store.Update(r.Context(), body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// Delete removes a blog. Only the owner may delete it.
func (h *blogHandler) Delete(w http.ResponseWriter, r *http.Request) {
	blog, ok := h.requireOwnedBlog(w, r)
	if !ok {
		return
	}

	if err := h.store.Delete(r.Context(), blog.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
