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

// blogSlugAttempts bounds how many suffixed slugs a post draws once some post already holds the
// plain form of its title. Each draw is one of about 28 million per title, so reaching even the
// second is unlikely and exhausting all of them is not something a real post will do.
const blogSlugAttempts = 5

// blogResponse is a blog as clients see it: the stored post, plus who wrote it. A username is the
// whole of an author's public identity, so it is carried directly rather than wrapped.
//
// It is what a client links an author by: a post is addressed by its slug alone, so the username
// here is the only handle it holds for the profile behind the post - a post records its owner by
// uid, which is never public. Resolving it on read rather than storing it on the post means
// renaming reaches every post at once instead of leaving old ones attributed to a name nobody
// holds. It is empty when the owner never created a profile, which a post does not require.
type blogResponse struct {
	entity.Blog
	AuthorUsername string `json:"authorUsername"`
}

// usernamesFor resolves the username behind each uid, looking up each distinct one once. That is
// what keeps a feed dominated by one author - or a comment thread dominated by one commenter -
// costing one read rather than one per row.
//
// A missing profile is an empty username rather than a failed request: neither posting nor
// commenting ever required one.
func (s *Service) usernamesFor(ctx context.Context, uids []string) (map[string]string, error) {
	usernames := make(map[string]string)
	for _, uid := range uids {
		if _, resolved := usernames[uid]; resolved {
			continue
		}

		user, err := s.users.Get(ctx, uid)
		if errors.Is(err, repository.ErrNotFound) {
			usernames[uid] = ""
			continue
		}
		if err != nil {
			return nil, err
		}
		usernames[uid] = user.Username
	}
	return usernames, nil
}

// authorsFor resolves the username behind each distinct owner in blogs.
func (s *Service) authorsFor(ctx context.Context, blogs []entity.Blog) (map[string]string, error) {
	uids := make([]string, 0, len(blogs))
	for _, blog := range blogs {
		uids = append(uids, blog.OwnerID)
	}
	return s.usernamesFor(ctx, uids)
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
// "hello-world". The plain slug is tried first, so the first post anywhere under a title reads as
// itself in the URL; only once some post already holds that slug does one fall back to a suffixed
// slug like "hello-world-k3m9x".
//
// Slugs are unique across every author (see the repository's document key), which is what lets a
// post be addressed by its slug with no author beside it - so a title is contended with every
// other post, not only the author's own. As with usernames, only the write can tell whether a slug
// is free, so a collision is answered by drawing another rather than by checking beforehand, which
// would be slower and still racy.
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
// attributed: a username is the only public handle an author has, and without one a post shows and
// links to nobody (see blogResponse). The client creates a profile for itself the first time it
// signs in, but nothing stops a caller reaching this endpoint before it ever has.
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

// blogFromPath loads the post named by the {slug} it is addressed by, writing a 404 and returning
// false if the slug is malformed or names nothing.
//
// A malformed slug answers as 404 rather than as the 400 SetSlug would give: the path is a URL a
// reader followed, not a form they filled in, and a rule it broke is exactly what the address of a
// private post looks like from the outside - so it is folded into the same "nothing here" every
// other kind of miss gets, rather than distinguished from them. A slug the frontend reserves
// (/post/new) is refused by SetSlug and so lands here too, which is right: no post holds one.
func (s *Service) blogFromPath(w http.ResponseWriter, r *http.Request) (entity.Blog, bool) {
	notFound := func() (entity.Blog, bool) {
		writeError(w, http.StatusNotFound, "blog not found")
		return entity.Blog{}, false
	}

	var candidate entity.Blog
	if err := candidate.SetSlug(r.PathValue("slug")); err != nil {
		return notFound()
	}

	blog, err := s.blogs.Get(r.Context(), candidate.Slug)
	if errors.Is(err, repository.ErrNotFound) {
		return notFound()
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return entity.Blog{}, false
	}
	return blog, true
}

// requireBlogPermission loads the post the request addresses and checks the caller holds the named
// permission on it (see entity.Blog.Permission), writing the error response and returning false
// otherwise.
//
// Readability is checked first whatever the action, and a caller who fails it is shown the same
// "not found" a missing post gets rather than the 403 they were really refused with: a URL built
// from the post's own title is exactly what a stranger can guess, and a 403 would confirm the
// guess. Only once a post is visible to the caller does a failed check become a 403, which reveals
// nothing they did not already know. Asking for ActionRead therefore checks the same thing twice
// and answers 404 either way, which is what makes requireReadableBlog below only a name for it.
func (s *Service) requireBlogPermission(w http.ResponseWriter, r *http.Request, action entity.Action) (entity.Blog, bool) {
	blog, ok := s.blogFromPath(w, r)
	if !ok {
		return entity.Blog{}, false
	}

	uid := uidFromContext(r.Context())
	if !blog.Permission(entity.ActionRead).Allows(uid) {
		writeError(w, http.StatusNotFound, "blog not found")
		return entity.Blog{}, false
	}
	if !blog.Permission(action).Allows(uid) {
		writeError(w, http.StatusForbidden, "forbidden")
		return entity.Blog{}, false
	}
	return blog, true
}

// requireReadableBlog loads the post the request addresses and checks the caller may read it.
func (s *Service) requireReadableBlog(w http.ResponseWriter, r *http.Request) (entity.Blog, bool) {
	return s.requireBlogPermission(w, r, entity.ActionRead)
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
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}

	response, err := s.withAuthor(r.Context(), blog)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// CreateBlog makes a new blog owned by the caller, addressed by a slug taken from its title (see
// saveBlog).
func (s *Service) CreateBlog(w http.ResponseWriter, r *http.Request) {
	blog := entity.Blog{OwnerID: uidFromContext(r.Context())}
	// A post is created owned by whoever asked, so the permission is asked of a post that already
	// names them: what it excludes is a caller with no uid, which requireAuth has already refused.
	// Asking anyway is what keeps creating a post in the same table as every other action rather
	// than being the one rule nothing writes down.
	if !requirePermission(w, r, blog.Permission(entity.ActionCreate)) {
		return
	}

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
	blog, ok := s.requireBlogPermission(w, r, entity.ActionUpdate)
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
	blog, ok := s.requireBlogPermission(w, r, entity.ActionDelete)
	if !ok {
		return
	}

	if err := s.blogs.Delete(r.Context(), blog.Slug); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
