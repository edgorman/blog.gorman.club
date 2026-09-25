package service

import (
	"errors"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	blogv1 "github.com/edgorman/blog.gorman.club/services/backend/internal/gen/blog/v1"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	// relatedPostsLimit is how many related posts a post shows.
	relatedPostsLimit = 3
	// relatedCandidates is how many neighbours are asked of the index: more than are shown, since
	// the post itself is among them and some may be posts the caller cannot read.
	relatedCandidates = 4 * relatedPostsLimit
)

// RelatedBlogs returns the posts nearest in meaning to the addressed one, closest first.
//
// The post's own read rule is asked first, so a caller who cannot read it gets the same 404 as
// GetBlog. The index then only ranks: every candidate is loaded and kept only if the caller may
// read it, exactly as a feed page is filtered, so a private post never surfaces here for someone
// who could not have opened it. A post the worker has not embedded yet has no neighbours, which is
// an empty list rather than an error.
func (s *Service) RelatedBlogs(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}
	if !requirePermission(w, r, entity.PermissionFor(entity.ResourceRelated, entity.ActionRead)) {
		return
	}
	internalError := func() { writeError(w, http.StatusInternalServerError, "internal error") }

	uid := uidFromContext(r.Context())
	related := make([]entity.Blog, 0, relatedPostsLimit)

	embedding, err := s.embeddings.Get(r.Context(), blog.Slug)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		internalError()
		return
	}
	if err == nil {
		slugs, err := s.embeddings.Nearest(r.Context(), embedding.Vector, relatedCandidates)
		if err != nil {
			internalError()
			return
		}
		for _, slug := range slugs {
			if len(related) == relatedPostsLimit {
				break
			}
			if slug == blog.Slug {
				continue
			}
			candidate, err := s.blogs.Get(r.Context(), slug)
			// An embedding can outlive its post until the worker catches up on the delete.
			if errors.Is(err, repository.ErrNotFound) {
				continue
			}
			if err != nil {
				internalError()
				return
			}
			if candidate.CanBeReadBy(uid) {
				related = append(related, candidate)
			}
		}
	}

	responses, err := s.withAuthors(r.Context(), related)
	if err != nil {
		internalError()
		return
	}
	posts := make([]*blogv1.Blog, 0, len(responses))
	for _, response := range responses {
		posts = append(posts, blogMessage(response))
	}
	writeProto(w, http.StatusOK, &blogv1.RelatedPosts{Posts: posts})
}
