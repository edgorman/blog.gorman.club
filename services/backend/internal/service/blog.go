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

// blogSlugAttempts bounds how many suffixed slugs a post draws once its author already holds the
// plain form of its title. Each draw is one of about 28 million per title, so reaching even the
// second is unlikely and exhausting all of them is not something a real post will do.
const blogSlugAttempts = 5

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

// saveBlog writes a new post under a slug derived from its title, e.g. "Hello, world!" at
// "hello-world". The plain slug is tried first, so an author's first post under a title reads as
// itself in the URL; only once they already hold that slug does a post fall back to a suffixed one
// like "hello-world-k3m9x".
//
// Slugs are scoped to the author (see the repository's document key), so a title is only ever
// contended with the author's own posts - two people may both hold "hello-world". As with
// usernames, only the write can tell whether a slug is free, so a collision is answered by drawing
// another rather than by checking beforehand, which would be slower and still racy.
func (s *Service) saveBlog(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	blog.Slug = entity.NewBlogSlug(blog.Title)

	created, err := s.blogs.Create(ctx, blog)
	if !errors.Is(err, repository.ErrSlugTaken) {
		return created, err
	}

	for range blogSlugAttempts {
		blog.Slug = entity.NewUniqueBlogSlug(blog.Title)

		if created, err = s.blogs.Create(ctx, blog); !errors.Is(err, repository.ErrSlugTaken) {
			return created, err
		}
	}
	// Deliberately not wrapped: running out of draws is the server failing to place a post, not the
	// caller asking for a slug somebody else holds - a client never chooses one at all.
	return entity.Blog{}, fmt.Errorf("no free slug after %d attempts: %v", blogSlugAttempts, err)
}

// ensureAuthor gives uid a profile if it has none, so that the post about to be written can be
// reached: a post is addressed as /blogs/{username}/{slug}, which an author with no username has
// no form of. The client creates a profile for itself the first time it signs in, but nothing
// stops a caller reaching this endpoint before it ever has.
func (s *Service) ensureAuthor(ctx context.Context, uid string) error {
	_, err := s.users.Get(ctx, uid)
	if !errors.Is(err, repository.ErrNotFound) {
		return err
	}

	// The profile is created with no username of its own, which is what has saveUser name it.
	_, err = s.saveUser(ctx, entity.User{ID: uid})
	return err
}

// blogRequest is the client-settable half of a blog. The slug, ownerId, and the timestamps are
// decided by the server, so they are absent here rather than decoded and then overwritten.
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

// blogFromPath loads the post named by the {username}/{slug} pair it is addressed by, writing the
// error response and returning false if either half is malformed or names nothing.
//
// The author is resolved through their profile because a post records its owner by uid, and a uid
// is never public: the username is the only handle a caller holds, and the uid behind it is what
// the post is keyed by. Validating both halves first answers a malformed one with the rule it
// broke, rather than with the 404 that looking it up would produce.
func (s *Service) blogFromPath(w http.ResponseWriter, r *http.Request) (entity.Blog, bool) {
	var author entity.User
	if err := author.SetUsername(r.PathValue("username")); err != nil {
		writeValidationError(w, err)
		return entity.Blog{}, false
	}
	var candidate entity.Blog
	if err := candidate.SetSlug(r.PathValue("slug")); err != nil {
		writeValidationError(w, err)
		return entity.Blog{}, false
	}

	// A username nobody holds and a username whose holder has no such post are the same 404: the
	// post asked for does not exist either way, and which half missed is not the caller's business.
	owner, err := s.users.GetByUsername(r.Context(), author.Username)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "blog not found")
		return entity.Blog{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return entity.Blog{}, false
	}

	blog, err := s.blogs.Get(r.Context(), owner.ID, candidate.Slug)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "blog not found")
		return entity.Blog{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return entity.Blog{}, false
	}
	return blog, true
}

// requireOwnedBlog loads the post the request addresses and checks the caller owns it, writing the
// error response and returning false otherwise.
func (s *Service) requireOwnedBlog(w http.ResponseWriter, r *http.Request) (entity.Blog, bool) {
	blog, ok := s.blogFromPath(w, r)
	if !ok {
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
	blog, ok := s.blogFromPath(w, r)
	if !ok {
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

// CreateBlog makes a new blog owned by the caller, addressed by their username and a slug taken
// from its title (see saveBlog).
func (s *Service) CreateBlog(w http.ResponseWriter, r *http.Request) {
	blog := entity.Blog{OwnerID: uidFromContext(r.Context())}
	if !decodeBlogRequest(w, r, &blog) {
		return
	}

	if err := s.ensureAuthor(r.Context(), blog.OwnerID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
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
// the request is applied to the stored blog, the slug, ownerId, and createdAt carry over untouched
// - so retitling a post changes what readers see without moving the URL it lives at.
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

	if err := s.blogs.Delete(r.Context(), blog.OwnerID, blog.Slug); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
