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

// blogIDAttempts bounds how many suffixed ids a post draws once the plain form of its title is
// taken. Each draw is one of about 28 million per title, so reaching even the second is unlikely
// and exhausting all of them is not something a real post will do.
const blogIDAttempts = 5

// blogResponse is a blog as clients see it: the stored post, plus who wrote it. A username is the
// whole of an author's public identity, so it is carried directly rather than wrapped.
//
// It is resolved on read rather than stored on the post, so renaming reaches every post at once
// instead of leaving old ones attributed to a name nobody holds, and a client never has to resolve
// one itself - which it could not do anyway, since a profile is only addressable by a username and
// a post records its owner by uid. It is empty when the owner never created a profile, which a
// post does not require.
type blogResponse struct {
	entity.Blog
	AuthorUsername string `json:"authorUsername"`
}

// authorsFor resolves the username behind each distinct owner in blogs. Looking up owners rather
// than posts means a feed dominated by one author costs one extra read, not one per post.
//
// A missing profile is an empty username rather than a failed request: posting never required one.
func (s *Service) authorsFor(ctx context.Context, blogs []entity.Blog) (map[string]string, error) {
	authors := make(map[string]string)
	for _, blog := range blogs {
		if _, resolved := authors[blog.OwnerID]; resolved {
			continue
		}

		user, err := s.users.Get(ctx, blog.OwnerID)
		if errors.Is(err, repository.ErrNotFound) {
			authors[blog.OwnerID] = ""
			continue
		}
		if err != nil {
			return nil, err
		}
		authors[blog.OwnerID] = user.Username
	}
	return authors, nil
}

// withAuthors pairs every blog with its owner's username.
func (s *Service) withAuthors(ctx context.Context, blogs []entity.Blog) ([]blogResponse, error) {
	authors, err := s.authorsFor(ctx, blogs)
	if err != nil {
		return nil, err
	}

	responses := make([]blogResponse, 0, len(blogs))
	for _, blog := range blogs {
		responses = append(responses, blogResponse{Blog: blog, AuthorUsername: authors[blog.OwnerID]})
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

// saveBlog writes a new post under an id derived from its title, e.g. "Hello, world!" at
// "hello-world". The plain slug is tried first, so the first post to use a title reads as itself
// in the URL; only once that is taken does a post fall back to a suffixed id like
// "hello-world-k3m9x".
//
// Ids are global rather than per-author, since a post is addressed at /blogs/{id} with no author
// in the path. As with usernames, only the write can tell whether one is free, so a collision is
// answered by drawing another rather than by checking beforehand, which would be slower and still
// racy.
func (s *Service) saveBlog(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	blog.ID = entity.NewBlogID(blog.Title)

	created, err := s.blogs.Create(ctx, blog)
	if !errors.Is(err, repository.ErrBlogIDTaken) {
		return created, err
	}

	for range blogIDAttempts {
		blog.ID = entity.NewUniqueBlogID(blog.Title)

		if created, err = s.blogs.Create(ctx, blog); !errors.Is(err, repository.ErrBlogIDTaken) {
			return created, err
		}
	}
	// Deliberately not wrapped: running out of draws is the server failing to place a post, not the
	// caller asking for an id somebody else holds - a client never chooses one at all.
	return entity.Blog{}, fmt.Errorf("no free blog id after %d attempts: %v", blogIDAttempts, err)
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

// CreateBlog makes a new blog owned by the caller, named after its title (see saveBlog).
func (s *Service) CreateBlog(w http.ResponseWriter, r *http.Request) {
	blog := entity.Blog{OwnerID: uidFromContext(r.Context())}
	if !decodeBlogRequest(w, r, &blog) {
		return
	}

	created, err := s.saveBlog(r.Context(), blog)
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
// the request is applied to the stored blog, the id, ownerId, and createdAt carry over untouched -
// so retitling a post changes what readers see without moving the URL it lives at.
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
