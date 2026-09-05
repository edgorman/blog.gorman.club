// Package cache decorates a repository with a short-lived, in-process cache of the reads that are
// the same for everybody, so the highest-traffic query in the service is not re-run per visitor.
//
// It is a decorator rather than a layer of its own: it implements the same repository interface it
// wraps, so nothing above it - the service, its handlers, its tests - knows whether an answer came
// from the datastore or from memory, and removing the cache is deleting one line in cmd/backend.
package cache

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// blogListTTL is how long a cached page of the anonymous feed is answered before it is fetched
// again, and so also the longest a freshly published post can be missing from the landing feed a
// stranger sees. It is deliberately short: the point here is to collapse the burst of identical
// reads a popular link produces, not to serve an old feed for minutes - a reader who follows a
// link to a brand new post still reads it immediately, since a post is fetched by slug and only
// the listing is cached.
const blogListTTL = 30 * time.Second

// maxBlogListEntries bounds how many pages are held at once, because the key includes the caller's
// filters and `q` is free text: without a cap, a caller sending a distinct search term per request
// would grow this map for as long as it kept sending them. Reaching the cap does not fail or evict
// a live page - it simply stops caching new ones until entries expire (see store) - so the
// pathological case degrades to the uncached behaviour rather than to unbounded memory. The volume
// of datastore reads it can provoke is bounded by the rate limiter in front of the API, not here.
const maxBlogListEntries = 128

// blogListEntry is one cached page: exactly what List answered, plus when that answer goes stale.
type blogListEntry struct {
	blogs   []entity.Blog
	hasMore bool
	expires time.Time
}

var _ repository.BlogRepository = (*BlogRepository)(nil)

// BlogRepository caches the anonymous feed in front of another repository.BlogRepository.
//
// Only the anonymous caller's pages are cached, and that restriction is what makes the cache safe
// rather than something to be careful with: a page is the posts entity.Blog.CanBeReadBy admits for
// one uid, so two callers share an answer only if they share a uid. The empty uid is the one that
// is not an account at all - CanBeReadBy grants it public posts and nothing else - so every
// anonymous caller's page really is the same page, while a signed-in caller's page carries their
// own private and whitelisted posts and is therefore never stored here and never served from here.
// There is no per-uid keying to get wrong, because there are no per-uid entries.
//
// The inner repository is held as a field rather than embedded: forwarding has to be written out
// so that a method added to repository.BlogRepository later fails to compile here until somebody
// decides whether it reads (and should be cached) or writes (and should invalidate), instead of
// being silently passed through.
type BlogRepository struct {
	inner repository.BlogRepository
	ttl   time.Duration
	// now is the clock, swappable so a test can expire an entry without sleeping.
	now func() time.Time

	mu      sync.Mutex
	entries map[string]blogListEntry
}

// NewBlogRepository returns inner with its anonymous listings cached for blogListTTL.
func NewBlogRepository(inner repository.BlogRepository) *BlogRepository {
	return &BlogRepository{
		inner:   inner,
		ttl:     blogListTTL,
		now:     time.Now,
		entries: make(map[string]blogListEntry),
	}
}

// blogListKey names one cached page by everything that shapes it. The uid is not part of it: only
// the anonymous caller reaches the cache at all, so every entry already belongs to the same
// (empty) uid, and including it would suggest otherwise.
//
// The parts are joined on a NUL rather than concatenated, so no value can spell the boundary
// between two of them and be read as a different query - a tag of "go" with an empty search term
// must not key the same page as an empty tag with a search term of "go".
//
// StartAfter is rendered in UTC because the same instant sent with two offsets is the same page,
// and time.Time is deliberately not used as a map key directly: its equality compares a location
// pointer and a monotonic reading as well as the instant, so two timestamps meaning the same
// moment can compare unequal.
//
// Limit is used as it arrived rather than clamped the way the datastore repository clamps it,
// which costs a duplicate entry for a caller asking for more posts than any page can hold. Copying
// the clamp here would be the more fragile choice: two implementations of one bound, only one of
// which is the one that actually decides the answer.
func blogListKey(params repository.ListParams) string {
	startAfter := ""
	if !params.StartAfter.IsZero() {
		startAfter = params.StartAfter.UTC().Format(time.RFC3339Nano)
	}
	return strings.Join([]string{
		strconv.Itoa(params.Limit),
		startAfter,
		params.OwnerUID,
		params.Tag,
		params.Query,
	}, "\x00")
}

// lookup answers a cached page if one is present and still fresh.
func (r *BlogRepository) lookup(key string) (blogListEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.entries[key]
	if !ok || !r.now().Before(entry.expires) {
		return blogListEntry{}, false
	}
	return entry, true
}

// store keeps a page until it expires, unless the cache is already full of unexpired ones.
//
// Expired entries are dropped here rather than by a goroutine or a timer, and only when the cap is
// actually in the way: an expired entry is never served (see lookup), so leaving it in the map
// costs a little memory and nothing else, while sweeping on every write would walk the whole map
// per request to reclaim entries that will be overwritten by their own key anyway.
func (r *BlogRepository) store(key string, entry blogListEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, held := r.entries[key]; !held && len(r.entries) >= maxBlogListEntries {
		r.sweep()
		if len(r.entries) >= maxBlogListEntries {
			return
		}
	}
	r.entries[key] = entry
}

// sweep drops every entry that has expired. Callers must hold r.mu.
func (r *BlogRepository) sweep() {
	now := r.now()
	for key, entry := range r.entries {
		if !now.Before(entry.expires) {
			delete(r.entries, key)
		}
	}
}

// invalidate forgets every cached page.
//
// It is deliberately all of them rather than the ones a written post appears in: a post can enter
// or leave a page by its visibility, its tags, its title, its body or its position in the feed, so
// working out which keys a write touches means re-deriving every filter the repository applies -
// a second implementation of List's rules, wrong in a way that shows up as a stale feed. Dropping
// everything costs at most one uncached read per distinct page after a write, and writes are rare
// next to the reads this exists for.
//
// This bounds staleness only within the process that served the write. Another instance serving
// the same feed still answers from its own memory until the entry expires, which is what makes the
// TTL, not this, the actual guarantee (see the README).
func (r *BlogRepository) invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()

	clear(r.entries)
}

// Get is not cached: it is a single document read by key, which is the cheapest thing Firestore
// does, and it answers a post whose readability depends on the caller.
func (r *BlogRepository) Get(ctx context.Context, slug string) (entity.Blog, error) {
	return r.inner.Get(ctx, slug)
}

// List answers the anonymous feed from memory when it holds a fresh page for these params, and
// otherwise reads through and keeps what it read. A signed-in caller is passed straight through in
// both directions: their page is theirs, so it is neither served from the cache nor written to it.
//
// A failed read is not cached - an error is not an answer, and caching one would extend a
// transient Firestore failure to every caller for the length of the TTL.
//
// Concurrent misses on the same key each read through rather than waiting on one another. That
// duplicates exactly the query the cache exists to avoid, so it is worth naming: it is bounded by
// how many requests arrive within one read rather than by traffic, and the alternative - holding
// the lock across the read, or a per-key single-flight - either serialises unrelated pages or adds
// machinery this scale does not yet earn.
func (r *BlogRepository) List(ctx context.Context, uid string, params repository.ListParams) ([]entity.Blog, bool, error) {
	if uid != "" {
		return r.inner.List(ctx, uid, params)
	}

	key := blogListKey(params)
	if entry, ok := r.lookup(key); ok {
		return slices.Clone(entry.blogs), entry.hasMore, nil
	}

	blogs, hasMore, err := r.inner.List(ctx, uid, params)
	if err != nil {
		return nil, false, err
	}

	// Cloned on the way in and again on the way out, so the slice a caller holds is never the one
	// the cache holds: without this, a caller reordering or truncating its page would be editing
	// what the next caller is served. The posts within are shared and treated as read-only, which
	// is how a listed post was already treated when every List built fresh ones.
	r.store(key, blogListEntry{blogs: slices.Clone(blogs), hasMore: hasMore, expires: r.now().Add(r.ttl)})
	return blogs, hasMore, nil
}

// Create writes through and drops the cache, so an author who publishes a post sees it in the feed
// this instance serves them rather than waiting out the TTL.
func (r *BlogRepository) Create(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	created, err := r.inner.Create(ctx, blog)
	if err != nil {
		return entity.Blog{}, err
	}
	r.invalidate()
	return created, nil
}

// Update writes through and drops the cache: an edit can change a post's title, body, tags or
// audience, all of which decide which pages it belongs to.
func (r *BlogRepository) Update(ctx context.Context, blog entity.Blog) (entity.Blog, error) {
	updated, err := r.inner.Update(ctx, blog)
	if err != nil {
		return entity.Blog{}, err
	}
	r.invalidate()
	return updated, nil
}

// Delete writes through and drops the cache, so a deleted post stops being listed at once rather
// than lingering in a page cached before it went.
func (r *BlogRepository) Delete(ctx context.Context, slug string) error {
	if err := r.inner.Delete(ctx, slug); err != nil {
		return err
	}
	r.invalidate()
	return nil
}
