// Package search decorates a blog repository so a search (`q`) is answered by meaning, from the
// post embeddings the worker keeps, rather than by the substring scan the datastore repository
// walks the feed with.
package search

import (
	"context"
	"errors"
	"log/slog"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

const (
	// maxResults caps a search, as maxBlogListPageSize caps a feed page.
	maxResults = 50
	// candidatesPerResult is how many neighbours are asked of the index per result wanted: more
	// than are returned, since some are posts the caller cannot read.
	candidatesPerResult = 3
	// filteredCandidates is how many are asked for when a tag or an owner also narrows the search,
	// the most FindNearest returns: those filters can drop most of the nearest posts, and asking for
	// too few would answer "nothing matches" when the posts are there, just further down.
	// ponytail: fine while the collection is a few hundred posts; past that, prefilter the
	// FindNearest by ownerId/tags instead.
	filteredCandidates = 1000
)

// QueryEmbedder turns a search query into a vector comparable with the stored post embeddings.
type QueryEmbedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

var _ repository.BlogRepository = (*BlogRepository)(nil)

// BlogRepository answers a List with a Query from the embeddings index, and passes every other
// call, and a List with no Query, to the repository it wraps.
//
// The inner repository is embedded rather than forwarded by hand, unlike the cache: only List
// reads differently here, and nothing else has anything to invalidate.
type BlogRepository struct {
	repository.BlogRepository
	embeddings repository.EmbeddingRepository
	embedder   QueryEmbedder
	logger     *slog.Logger
}

// NewBlogRepository returns inner with its searches answered by meaning.
func NewBlogRepository(inner repository.BlogRepository, embeddings repository.EmbeddingRepository, embedder QueryEmbedder, logger *slog.Logger) *BlogRepository {
	return &BlogRepository{BlogRepository: inner, embeddings: embeddings, embedder: embedder, logger: logger}
}

// List answers a search with the posts nearest in meaning to the query, most relevant first.
//
// Relevance has no createdAt to continue from, so a search is one page: the top Limit results and
// hasMore false. A StartAfter can therefore only be continuing a page the fallback below served,
// so it goes to the fallback too. The index only
// ranks - every candidate is loaded and kept only if uid may read it and it passes the owner and tag
// filters, exactly as a feed page is filtered - so a search still never widens what a caller sees.
//
// If the query cannot be embedded, or the index cannot be read, the search falls back to the inner
// repository's substring scan rather than failing: a worse answer beats none.
func (r *BlogRepository) List(ctx context.Context, uid string, params repository.ListParams) ([]entity.Blog, bool, error) {
	if params.Query == "" || !params.StartAfter.IsZero() {
		return r.BlogRepository.List(ctx, uid, params)
	}
	limit := params.Limit
	if limit <= 0 || limit > maxResults {
		limit = maxResults
	}

	vector, err := r.embedder.EmbedQuery(ctx, params.Query)
	if err != nil {
		r.logger.Warn("search fell back to substring scan: embedding the query failed", "error", err)
		return r.BlogRepository.List(ctx, uid, params)
	}
	candidates := candidatesPerResult * limit
	if params.Tag != "" || params.OwnerUID != "" {
		candidates = filteredCandidates
	}
	slugs, err := r.embeddings.Nearest(ctx, vector, candidates)
	if err != nil {
		r.logger.Warn("search fell back to substring scan: nearest-neighbour query failed", "error", err)
		return r.BlogRepository.List(ctx, uid, params)
	}

	// ponytail: one Get per candidate, in order, stopping once the page is full; batch them if
	// search latency starts to show.
	blogs := make([]entity.Blog, 0, limit)
	for _, slug := range slugs {
		if len(blogs) == limit {
			break
		}
		blog, err := r.BlogRepository.Get(ctx, slug)
		// An embedding can outlive its post until the worker catches up on the delete.
		if errors.Is(err, repository.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, false, err
		}
		if !blog.CanBeReadBy(uid) {
			continue
		}
		if params.OwnerUID != "" && blog.OwnerID != params.OwnerUID {
			continue
		}
		if params.Tag != "" && !blog.HasTag(params.Tag) {
			continue
		}
		blogs = append(blogs, blog)
	}
	return blogs, false, nil
}
