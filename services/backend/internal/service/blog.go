package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// blogAuthor is the public half of a post's owner, carried on the post so a client never has to
// resolve one itself - which it could not do anyway, since a profile is only addressable by a
// username and a post records its owner by uid.
type blogAuthor struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// blogResponse is a blog as clients see it: the stored post, plus who wrote it. The author is
// resolved on read rather than stored on the post, so renaming reaches every post at once instead
// of leaving old ones attributed to a name nobody holds. Author is null when the owner never
// created a profile, which a post does not require.
type blogResponse struct {
	entity.Blog
	Author *blogAuthor `json:"author"`
}

// authorsFor resolves the profile behind each distinct owner in blogs. Looking up owners rather
// than posts means a feed dominated by one author costs one extra read, not one per post.
//
// A missing profile is a nil author rather than a failed request: posting never required one.
func (s *Service) authorsFor(ctx context.Context, blogs []entity.Blog) (map[string]*blogAuthor, error) {
	authors := make(map[string]*blogAuthor)
	for _, blog := range blogs {
		if _, resolved := authors[blog.OwnerID]; resolved {
			continue
		}

		user, err := s.users.Get(ctx, blog.OwnerID)
		if errors.Is(err, repository.ErrNotFound) {
			authors[blog.OwnerID] = nil
			continue
		}
		if err != nil {
			return nil, err
		}
		authors[blog.OwnerID] = &blogAuthor{Username: user.Username, DisplayName: user.DisplayName}
	}
	return authors, nil
}

// withAuthors pairs every blog with its owner's profile.
func (s *Service) withAuthors(ctx context.Context, blogs []entity.Blog) ([]blogResponse, error) {
	authors, err := s.authorsFor(ctx, blogs)
	if err != nil {
		return nil, err
	}

	responses := make([]blogResponse, 0, len(blogs))
	for _, blog := range blogs {
		responses = append(responses, blogResponse{Blog: blog, Author: authors[blog.OwnerID]})
	}
	return responses, nil
}

// withAuthor is withAuthors for the handlers that answer with a single post.
func (s *Service) withAuthor(ctx context.Context, blog entity.Blog) (blogResponse, error) {
	responses, err := s.withAuthors(ctx, []entity.Blog{blog})
	if err != nil {
		return blogResponse{}, err
	}
	return responses[0], nil
}

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
	// withAuthors always builds its slice, so an empty collection stays an empty JSON array rather
	// than becoming null - which is what the nil check that used to sit here was for.
	responses, err := s.withAuthors(r.Context(), blogs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, responses)
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

	response, err := s.withAuthor(r.Context(), blog)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, response)
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

	response, err := s.withAuthor(r.Context(), created)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, response)
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

	response, err := s.withAuthor(r.Context(), updated)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, response)
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
