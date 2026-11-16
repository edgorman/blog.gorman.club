package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/api"
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
	AuthorUsername string
}

// blogMessage is the wire shape of a single post: what GetBlog, CreateBlog, and UpdateBlog answer
// with, and what each entry of a BlogPage carries. DeletedAt has no field on api.Blog at all, so a
// soft-deleted post - which readAccess already keeps out of every response - could not leak through
// here even by oversight. Times are UTC, so they always render with a "Z", and the lists are never
// nil, so an empty one is `[]` rather than `null`.
func blogMessage(response blogResponse) api.Blog {
	return api.Blog{
		Slug:           response.Slug,
		AuthorUsername: response.AuthorUsername,
		Title:          response.Title,
		Content:        response.Content,
		Tags:           nonNil(response.Tags),
		Visibility:     api.Visibility(response.Visibility),
		AllowedUserIds: nonNil(response.AllowedUserIDs),
		CreatedAt:      response.CreatedAt.UTC(),
		UpdatedAt:      response.UpdatedAt.UTC(),
	}
}

// nonNil is items, or an empty slice when there are none, so it encodes as `[]` rather than `null`.
func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// blogMessages converts each post the same way blogMessage does.
func blogMessages(responses []blogResponse) []api.Blog {
	posts := make([]api.Blog, 0, len(responses))
	for _, response := range responses {
		posts = append(posts, blogMessage(response))
	}
	return posts
}

// blogPageMessage is one page of ListBlogs.
func blogPageMessage(responses []blogResponse, hasMore bool) api.BlogPage {
	return api.BlogPage{Posts: blogMessages(responses), HasMore: hasMore}
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

// withAuthors pairs every blog with its owner's username. It is also where a post's whitelist is
// withheld from everybody but its owner: who a post was shared with is the owner's business, not
// that of the readers named on it, nor of anybody reading a public post that still carries one.
func (s *Service) withAuthors(ctx context.Context, blogs []entity.Blog) ([]blogResponse, error) {
	authors, err := s.authorsFor(ctx, blogs)
	if err != nil {
		return nil, err
	}

	uid := uidFromContext(ctx)
	responses := make([]blogResponse, 0, len(blogs))
	for _, blog := range blogs {
		if !blog.IsOwnedBy(uid) {
			blog.AllowedUserIDs = nil
		}
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

	// The profile is created with no username of its own, which is what has saveUser name it. It is
	// created rather than put, so a profile the client wrote since the lookup above - its first
	// sign-in racing this post - keeps the name and bio it chose.
	_, err = s.saveUser(ctx, entity.User{ID: uid}, s.users.Create)
	if errors.Is(err, repository.ErrUserExists) {
		return nil
	}
	return err
}

// applyBlogRequest validates every field through the entity's setters before touching blog. The
// slug, ownerId, and the timestamps are decided by the server, so api.BlogRequest carries none
// of them - they are left untouched on blog rather than decoded and then overwritten.
func applyBlogRequest(req *api.BlogRequest, blog *entity.Blog) error {
	candidate := *blog
	if err := candidate.SetTitle(req.Title); err != nil {
		return err
	}
	if err := candidate.SetContent(req.Content); err != nil {
		return err
	}
	if err := candidate.SetTags(req.Tags); err != nil {
		return err
	}
	// Visibility and the whitelist below are what a post's read audience is made of; Tags above is
	// only what it is filed under, and narrows a feed rather than widening who may see one.
	if err := candidate.SetVisibility(entity.Visibility(req.Visibility)); err != nil {
		return err
	}
	if err := candidate.SetAllowedUserIDs(req.AllowedUserIds); err != nil {
		return err
	}

	*blog = candidate
	return nil
}

// decodeBlogRequest reads a blog body and applies it to blog, writing the error response and
// returning false if it's malformed.
func decodeBlogRequest(w http.ResponseWriter, r *http.Request, blog *entity.Blog) bool {
	var body api.BlogRequest
	if !readJSON(w, r, &body) {
		return false
	}
	if err := applyBlogRequest(&body, blog); err != nil {
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
func (s *Service) blogFromPath(w http.ResponseWriter, r *http.Request, slug api.Slug) (entity.Blog, bool) {
	notFound := func() (entity.Blog, bool) {
		writeError(w, http.StatusNotFound, "blog not found")
		return entity.Blog{}, false
	}

	var candidate entity.Blog
	if err := candidate.SetSlug(slug); err != nil {
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
func (s *Service) requireBlogPermission(w http.ResponseWriter, r *http.Request, slug api.Slug, action entity.Action) (entity.Blog, bool) {
	blog, ok := s.blogFromPath(w, r, slug)
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
func (s *Service) requireReadableBlog(w http.ResponseWriter, r *http.Request, slug api.Slug) (entity.Blog, bool) {
	return s.requireBlogPermission(w, r, slug, entity.ActionRead)
}

// listBlogsDefaultLimit is the page size ListBlogs applies when a caller sends no `limit`, keeping
// even a bare `GET /blogs` bounded rather than defaulting to the unbounded read pagination exists
// to avoid.
const listBlogsDefaultLimit = 20

// listParams turns ListBlogs' paging, scope and filters into the repository's: `limit` bounds the
// page, `startAfter` continues one, `tag` narrows to one topic, and `q` narrows to posts holding a
// term. Each is optional, and a malformed one is reported as a 400 rather than silently ignored -
// a client sending garbage here is the one case among List's callers where that garbage should not
// be read as "no preference".
//
// They compose rather than exclude each other, because each answers a different question and a
// reader asking two at once ("posts of mine about Go") means both.
func listParams(query api.ListBlogsParams) (repository.ListParams, error) {
	params := repository.ListParams{
		Limit: listBlogsDefaultLimit,
		Query: strings.TrimSpace(deref(query.Q)),
	}

	// The tag is normalized here rather than in the repository, so what reaches Firestore is
	// already the one form a post stores it in. A tag that normalizes away was still asked for -
	// answering the unfiltered feed to it would silently ignore the filter - so it is a 400.
	if raw := deref(query.Tag); strings.TrimSpace(raw) != "" {
		params.Tag = entity.NormalizeTag(raw)
		if params.Tag == "" {
			return repository.ListParams{}, errors.New("tag must hold at least one letter or digit")
		}
	}

	if query.Limit != nil {
		if *query.Limit <= 0 {
			return repository.ListParams{}, errors.New("limit must be a positive integer")
		}
		params.Limit = *query.Limit
	}

	if query.StartAfter != nil {
		params.StartAfter = *query.StartAfter
	}

	return params, nil
}

// deref is the value a pointer holds, or the zero value when it is absent.
func deref[T any](value *T) T {
	var zero T
	if value == nil {
		return zero
	}
	return *value
}

// ListBlogs returns one page of the blogs the caller is allowed to read, newest first - the whole
// collection for the general feed, or narrowed by `author` (a username) to one author's posts, by `tag` to one
// topic, and by `q` to posts holding a term.
//
// Filtering never widens what comes back: every narrowing is applied on top of the same
// entity.Blog.CanBeReadBy the unfiltered feed uses, so searching cannot surface a post a reader
// could not have found by scrolling.
//
// A search calls a paid embedding model (see internal/repository/search), so it is metered on its
// own budget as well, by account or by address for a caller who is not signed in.
func (s *Service) ListBlogs(w http.ResponseWriter, r *http.Request, query api.ListBlogsParams) {
	searchLimited(s.searchLimiter, func(w http.ResponseWriter, r *http.Request) {
		s.listBlogs(w, r, query)
	})(w, r)
}

func (s *Service) listBlogs(w http.ResponseWriter, r *http.Request, query api.ListBlogsParams) {
	params, err := listParams(query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// A profile feed names its author by username, the only handle a client holds for one, and is
	// resolved to the uid posts are stored under here. A name nobody holds has no posts.
	if author := strings.TrimSpace(deref(query.Author)); author != "" {
		user, err := s.users.GetByUsername(r.Context(), author)
		if errors.Is(err, repository.ErrNotFound) {
			writeJSON(w, http.StatusOK, blogPageMessage(nil, false))
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		params.OwnerUID = user.ID
	}

	blogs, hasMore, err := s.blogs.List(r.Context(), uidFromContext(r.Context()), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	responses, err := s.withAuthors(r.Context(), blogs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, blogPageMessage(responses, hasMore))
}

// GetBlog returns a single blog, provided the caller is allowed to read it.
func (s *Service) GetBlog(w http.ResponseWriter, r *http.Request, slug api.Slug) {
	blog, ok := s.requireReadableBlog(w, r, slug)
	if !ok {
		return
	}

	response, err := s.withAuthor(r.Context(), blog)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, blogMessage(response))
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

	writeJSON(w, http.StatusCreated, blogMessage(response))
}

// UpdateBlog replaces a blog's client-settable fields. Only the owner may update it, and because
// the request is applied to the stored blog, the slug, ownerId, and createdAt carry over untouched
// - so retitling a post changes what readers see without moving the URL it lives at.
func (s *Service) UpdateBlog(w http.ResponseWriter, r *http.Request, slug api.Slug) {
	blog, ok := s.requireBlogPermission(w, r, slug, entity.ActionUpdate)
	if !ok {
		return
	}
	if !decodeBlogRequest(w, r, &blog) {
		return
	}

	updated, err := s.blogs.Update(r.Context(), blog)
	if err != nil {
		writeUpdateError(w, err)
		return
	}

	response, err := s.withAuthor(r.Context(), updated)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, blogMessage(response))
}

// writeUpdateError answers a failed BlogRepository.Update: a post deleted since it was read is gone
// as far as the caller is concerned, and one written since is a conflict to reload and retry,
// rather than something to overwrite.
func writeUpdateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "blog not found")
	case errors.Is(err, repository.ErrPostChanged):
		writeError(w, http.StatusConflict, "the post was changed elsewhere; reload it and try again")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

// DeleteBlog removes a blog. Only the owner may delete it.
func (s *Service) DeleteBlog(w http.ResponseWriter, r *http.Request, slug api.Slug) {
	blog, ok := s.requireBlogPermission(w, r, slug, entity.ActionDelete)
	if !ok {
		return
	}

	if err := s.blogs.Delete(r.Context(), blog.Slug); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
