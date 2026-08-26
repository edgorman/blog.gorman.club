package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// blogRequest is the client-settable half of a blog. ID, ownerId, and the timestamps are decided
// by the server, so they are absent here rather than decoded and then overwritten.
type blogRequest struct {
	Title          string            `json:"title"`
	Content        string            `json:"content"`
	Visibility     entity.Visibility `json:"visibility"`
	AllowedUserIDs []string          `json:"allowedUserIds"`
}

// applyTo validates every field through the entity's setters before touching blog.
func (b blogRequest) applyTo(blog *entity.Blog) error {
	candidate := *blog
	if err := candidate.SetTitle(b.Title); err != nil {
		return err
	}
	if err := candidate.SetContent(b.Content); err != nil {
		return err
	}
	if err := candidate.SetVisibility(b.Visibility); err != nil {
		return err
	}
	if err := candidate.SetAllowedUserIDs(b.AllowedUserIDs); err != nil {
		return err
	}

	*blog = candidate
	return nil
}

// decodeBlogRequest reads a blog body and applies it to blog, writing the error response and
// returning false if it's malformed.
func decodeBlogRequest(w http.ResponseWriter, r *http.Request, blog *entity.Blog) bool {
	var body blogRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	if err := body.applyTo(blog); err != nil {
		writeValidationError(w, err)
		return false
	}
	return true
}

// requireOwnedBlog loads the blog named by the {id} path value and checks the caller owns it,
// writing the error response and returning false otherwise.
func (s *Service) requireOwnedBlog(w http.ResponseWriter, r *http.Request) (entity.Blog, bool) {
	blog, err := s.blogs.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "blog not found")
		return entity.Blog{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return entity.Blog{}, false
	}
	if !blog.IsOwnedBy(uidFromContext(r.Context())) {
		writeError(w, http.StatusForbidden, "forbidden")
		return entity.Blog{}, false
	}
	return blog, true
}

// ListBlogs returns every blog the caller is allowed to read, newest first.
func (s *Service) ListBlogs(w http.ResponseWriter, r *http.Request) {
	blogs, err := s.blogs.List(r.Context(), uidFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if blogs == nil {
		// An empty collection is an empty JSON array, never null.
		blogs = []entity.Blog{}
	}

	writeJSON(w, http.StatusOK, blogs)
}

// GetBlog returns a single blog, provided the caller is allowed to read it.
func (s *Service) GetBlog(w http.ResponseWriter, r *http.Request) {
	blog, err := s.blogs.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "blog not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if !blog.CanBeReadBy(uidFromContext(r.Context())) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, blog)
}

// CreateBlog makes a new blog owned by the caller.
func (s *Service) CreateBlog(w http.ResponseWriter, r *http.Request) {
	blog := entity.Blog{OwnerID: uidFromContext(r.Context())}
	if !decodeBlogRequest(w, r, &blog) {
		return
	}

	created, err := s.blogs.Create(r.Context(), blog)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// UpdateBlog replaces a blog's client-settable fields. Only the owner may update it, and because
// the request is applied to the stored blog, ownerId and createdAt carry over untouched.
func (s *Service) UpdateBlog(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireOwnedBlog(w, r)
	if !ok {
		return
	}
	if !decodeBlogRequest(w, r, &blog) {
		return
	}

	updated, err := s.blogs.Update(r.Context(), blog)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// DeleteBlog removes a blog. Only the owner may delete it.
func (s *Service) DeleteBlog(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireOwnedBlog(w, r)
	if !ok {
		return
	}

	if err := s.blogs.Delete(r.Context(), blog.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
