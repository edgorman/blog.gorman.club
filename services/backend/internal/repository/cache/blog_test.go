package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// fakeBlogRepository is the repository the cache wraps, reduced to what these tests assert on:
// what it was asked, how often, and what it answers. It is not a working blog store - the cache
// never inspects a post, only whether it had to ask for one - so List returns a canned page rather
// than filtering anything.
type fakeBlogRepository struct {
	// mu guards the counters alone, so that the one test driving this from several goroutines is
	// exercising the cache's locking rather than tripping over the fake's.
	mu     sync.Mutex
	lists  int
	uids   []string
	params []repository.ListParams

	blogs   []entity.Blog
	hasMore bool
	err     error

	creates  int
	updates  int
	deletes  int
	writeErr error
}

func (r *fakeBlogRepository) Get(context.Context, string) (entity.Blog, error) {
	return entity.Blog{}, repository.ErrNotFound
}

func (r *fakeBlogRepository) List(_ context.Context, uid string, params repository.ListParams) ([]entity.Blog, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lists++
	r.uids = append(r.uids, uid)
	r.params = append(r.params, params)
	if r.err != nil {
		return nil, false, r.err
	}
	return r.blogs, r.hasMore, nil
}

func (r *fakeBlogRepository) Create(_ context.Context, blog entity.Blog) (entity.Blog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.creates++
	return blog, r.writeErr
}

func (r *fakeBlogRepository) Update(_ context.Context, blog entity.Blog) (entity.Blog, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.updates++
	return blog, r.writeErr
}

func (r *fakeBlogRepository) Delete(context.Context, string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deletes++
	return r.writeErr
}

// newTestCache wires a cache over inner with a clock the test drives, so an entry can be expired
// without waiting out the real TTL.
func newTestCache(inner *fakeBlogRepository) (*BlogRepository, *time.Time) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cached := NewBlogRepository(inner)
	cached.now = func() time.Time { return clock }
	return cached, &clock
}

func page(slugs ...string) []entity.Blog {
	blogs := make([]entity.Blog, 0, len(slugs))
	for _, slug := range slugs {
		blogs = append(blogs, entity.Blog{Slug: slug, Visibility: entity.VisibilityPublic})
	}
	return blogs
}

func listAnonymously(t *testing.T, cached *BlogRepository, params repository.ListParams) []entity.Blog {
	t.Helper()

	blogs, _, err := cached.List(context.Background(), "", params)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return blogs
}

func slugsOf(blogs []entity.Blog) []string {
	slugs := make([]string, 0, len(blogs))
	for _, blog := range blogs {
		slugs = append(slugs, blog.Slug)
	}
	return slugs
}

func assertSlugs(t *testing.T, blogs []entity.Blog, want ...string) {
	t.Helper()

	got := slugsOf(blogs)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("blogs = %v, want %v", got, want)
	}
}

func assertLists(t *testing.T, inner *fakeBlogRepository, want int) {
	t.Helper()

	if inner.lists != want {
		t.Fatalf("inner listed %d times, want %d", inner.lists, want)
	}
}

func TestList_AnonymousPageServedFromCache(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("first", "second"), hasMore: true}
	cached, _ := newTestCache(inner)

	first := listAnonymously(t, cached, repository.ListParams{Limit: 20})
	blogs, hasMore, err := cached.List(context.Background(), "", repository.ListParams{Limit: 20})
	if err != nil {
		t.Fatalf("second list: %v", err)
	}

	assertLists(t, inner, 1)
	assertSlugs(t, first, "first", "second")
	assertSlugs(t, blogs, "first", "second")
	if !hasMore {
		t.Fatal("hasMore = false, want the cached page to carry it back")
	}
}

// A signed-in caller's page is theirs: it can hold posts nobody else may read, so it must never be
// stored, and the anonymous page must never be handed to them either.
func TestList_SignedInCallerIsNeverCached(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("public")}
	cached, _ := newTestCache(inner)

	if _, _, err := cached.List(context.Background(), "caller", repository.ListParams{Limit: 20}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if _, _, err := cached.List(context.Background(), "caller", repository.ListParams{Limit: 20}); err != nil {
		t.Fatalf("second list: %v", err)
	}

	assertLists(t, inner, 2)
	if len(cached.entries) != 0 {
		t.Fatalf("cached %d entries for a signed-in caller, want none", len(cached.entries))
	}
}

// The reverse of the case above: a page warmed by an anonymous request is not what a signed-in
// caller gets back, since theirs may hold posts the anonymous one could not.
func TestList_SignedInCallerIsNotServedTheAnonymousPage(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("public")}
	cached, _ := newTestCache(inner)

	listAnonymously(t, cached, repository.ListParams{Limit: 20})
	if _, _, err := cached.List(context.Background(), "caller", repository.ListParams{Limit: 20}); err != nil {
		t.Fatalf("list: %v", err)
	}

	assertLists(t, inner, 2)
	if inner.uids[1] != "caller" {
		t.Fatalf("inner asked for uid %q, want the signed-in caller", inner.uids[1])
	}
}

// Each set of filters is its own page, so a search or a tag cannot be answered with the feed that
// was cached before it.
func TestList_ParamsKeyTheirOwnPage(t *testing.T) {
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	variants := []repository.ListParams{
		{Limit: 20},
		{Limit: 10},
		{Limit: 20, StartAfter: started},
		{Limit: 20, OwnerUID: "author"},
		{Limit: 20, Tag: "go"},
		{Limit: 20, Query: "go"},
	}

	inner := &fakeBlogRepository{blogs: page("first")}
	cached, _ := newTestCache(inner)

	for _, params := range variants {
		listAnonymously(t, cached, params)
	}
	for _, params := range variants {
		listAnonymously(t, cached, params)
	}

	assertLists(t, inner, len(variants))
}

// A tag and a search term must not collide just because their values could be concatenated the
// same way - which is what the NUL between the key's parts is for.
func TestList_FiltersCannotSpellEachOther(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("first")}
	cached, _ := newTestCache(inner)

	listAnonymously(t, cached, repository.ListParams{Tag: "go", Query: ""})
	listAnonymously(t, cached, repository.ListParams{Tag: "", Query: "go"})

	assertLists(t, inner, 2)
}

// The same instant sent with a different offset is the same page, so it is keyed as one.
func TestList_CursorIsKeyedByInstant(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("first")}
	cached, _ := newTestCache(inner)

	utc := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	offset := utc.In(time.FixedZone("elsewhere", 2*60*60))

	listAnonymously(t, cached, repository.ListParams{Limit: 20, StartAfter: utc})
	listAnonymously(t, cached, repository.ListParams{Limit: 20, StartAfter: offset})

	assertLists(t, inner, 1)
}

func TestList_EntryExpires(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("first")}
	cached, clock := newTestCache(inner)

	listAnonymously(t, cached, repository.ListParams{Limit: 20})
	*clock = clock.Add(blogListTTL - time.Millisecond)
	listAnonymously(t, cached, repository.ListParams{Limit: 20})
	assertLists(t, inner, 1)

	*clock = clock.Add(time.Millisecond)
	inner.blogs = page("second")
	assertSlugs(t, listAnonymously(t, cached, repository.ListParams{Limit: 20}), "second")
	assertLists(t, inner, 2)
}

func TestList_ErrorIsNotCached(t *testing.T) {
	failure := errors.New("firestore unavailable")
	inner := &fakeBlogRepository{err: failure}
	cached, _ := newTestCache(inner)

	if _, _, err := cached.List(context.Background(), "", repository.ListParams{Limit: 20}); !errors.Is(err, failure) {
		t.Fatalf("err = %v, want %v", err, failure)
	}

	inner.err = nil
	inner.blogs = page("first")
	assertSlugs(t, listAnonymously(t, cached, repository.ListParams{Limit: 20}), "first")
	assertLists(t, inner, 2)
}

// A caller holds its own slice: editing the page it was handed must not edit what the next caller
// is served.
func TestList_CallersDoNotShareTheCachedSlice(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("first", "second")}
	cached, _ := newTestCache(inner)

	first := listAnonymously(t, cached, repository.ListParams{Limit: 20})
	first[0] = entity.Blog{Slug: "tampered"}

	assertSlugs(t, listAnonymously(t, cached, repository.ListParams{Limit: 20}), "first", "second")
	assertLists(t, inner, 1)
}

// The repository the cache wraps holds its own slice too, so a page it keeps building on is not
// something the cache is quietly aliasing.
func TestList_CacheDoesNotAliasTheInnerPage(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("first", "second")}
	cached, _ := newTestCache(inner)

	listAnonymously(t, cached, repository.ListParams{Limit: 20})
	inner.blogs[0] = entity.Blog{Slug: "changed"}

	assertSlugs(t, listAnonymously(t, cached, repository.ListParams{Limit: 20}), "first", "second")
}

func TestWrites_InvalidateTheCache(t *testing.T) {
	blog := entity.Blog{Slug: "first", OwnerID: "author", Title: "First", Content: "Body", Visibility: entity.VisibilityPublic}
	writes := map[string]func(*BlogRepository) error{
		"create": func(cached *BlogRepository) error {
			_, err := cached.Create(context.Background(), blog)
			return err
		},
		"update": func(cached *BlogRepository) error {
			_, err := cached.Update(context.Background(), blog)
			return err
		},
		"delete": func(cached *BlogRepository) error {
			return cached.Delete(context.Background(), blog.Slug)
		},
	}

	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			inner := &fakeBlogRepository{blogs: page("first")}
			cached, _ := newTestCache(inner)

			listAnonymously(t, cached, repository.ListParams{Limit: 20})
			if err := write(cached); err != nil {
				t.Fatalf("write: %v", err)
			}

			inner.blogs = page("first", "second")
			assertSlugs(t, listAnonymously(t, cached, repository.ListParams{Limit: 20}), "first", "second")
			assertLists(t, inner, 2)
		})
	}
}

// A write that failed changed nothing, so it must not throw away pages that are still accurate.
func TestWrites_FailedWriteKeepsTheCache(t *testing.T) {
	failure := errors.New("firestore unavailable")
	inner := &fakeBlogRepository{blogs: page("first"), writeErr: failure}
	cached, _ := newTestCache(inner)

	listAnonymously(t, cached, repository.ListParams{Limit: 20})
	if err := cached.Delete(context.Background(), "first"); !errors.Is(err, failure) {
		t.Fatalf("err = %v, want %v", err, failure)
	}

	listAnonymously(t, cached, repository.ListParams{Limit: 20})
	assertLists(t, inner, 1)
}

// A caller sending a distinct search term per request must not be able to grow the cache without
// bound; once it is full of live pages, new ones are simply not kept.
func TestList_EntriesAreBounded(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("first")}
	cached, clock := newTestCache(inner)

	for i := range maxBlogListEntries * 2 {
		listAnonymously(t, cached, repository.ListParams{Limit: 20, Query: fmt.Sprintf("term-%d", i)})
	}
	if len(cached.entries) > maxBlogListEntries {
		t.Fatalf("held %d entries, want at most %d", len(cached.entries), maxBlogListEntries)
	}

	// Once the entries it filled up with have expired, the cache takes new ones again rather than
	// staying full forever.
	*clock = clock.Add(blogListTTL)
	listAnonymously(t, cached, repository.ListParams{Limit: 20, Query: "afterwards"})
	listAnonymously(t, cached, repository.ListParams{Limit: 20, Query: "afterwards"})
	assertLists(t, inner, maxBlogListEntries*2+1)
}

// One instance serves every request at once, so the cache is shared mutable state and its map is
// read, written and cleared concurrently. What this asserts is mostly the race detector's to make;
// the count only pins down that the page really was cached rather than every goroutine reading
// through.
func TestList_ConcurrentCallersShareTheCacheSafely(t *testing.T) {
	inner := &fakeBlogRepository{blogs: page("first")}
	cached := NewBlogRepository(inner)

	var readers sync.WaitGroup
	for i := range 64 {
		readers.Add(1)
		go func() {
			defer readers.Done()

			if _, _, err := cached.List(context.Background(), "", repository.ListParams{Limit: 20}); err != nil {
				t.Errorf("list: %v", err)
			}
			// Every eighth caller writes, so the readers above are racing invalidation as well as
			// each other.
			if i%8 == 0 {
				if err := cached.Delete(context.Background(), "first"); err != nil {
					t.Errorf("delete: %v", err)
				}
			}
		}()
	}
	readers.Wait()

	if inner.lists > 64 {
		t.Fatalf("inner listed %d times, want no more than one per caller", inner.lists)
	}
	if inner.deletes != 8 {
		t.Fatalf("inner deleted %d times, want 8", inner.deletes)
	}
}
