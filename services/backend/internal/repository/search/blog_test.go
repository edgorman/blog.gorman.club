package search

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// blogs is an inner repository holding posts by slug. Its List is the substring scan, recorded so a
// test can tell a fallback from an answer by meaning.
type blogs struct {
	repository.BlogRepository
	posts  map[string]entity.Blog
	listed bool
}

func (b *blogs) Get(_ context.Context, slug string) (entity.Blog, error) {
	post, ok := b.posts[slug]
	if !ok {
		return entity.Blog{}, repository.ErrNotFound
	}
	return post, nil
}

func (b *blogs) List(_ context.Context, uid string, params repository.ListParams) ([]entity.Blog, bool, error) {
	b.listed = true
	var out []entity.Blog
	for _, post := range b.posts {
		if post.CanBeReadBy(uid) && post.MatchesQuery(params.Query) {
			out = append(out, post)
		}
	}
	return out, false, nil
}

// index ranks stored vectors by real cosine distance, as Firestore's FindNearest does.
type index struct {
	repository.EmbeddingRepository
	vectors map[string][]float32
	err     error
}

func (i index) Nearest(_ context.Context, vector []float32, limit int) ([]string, error) {
	if i.err != nil {
		return nil, i.err
	}
	var slugs []string
	for slug := range i.vectors {
		slugs = append(slugs, slug)
	}
	distance := func(slug string) float64 { return 1 - cosine(vector, i.vectors[slug]) }
	slices.SortFunc(slugs, func(a, b string) int { return int(math.Copysign(1, distance(a)-distance(b))) })
	return slugs[:min(limit, len(slugs))], nil
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for k := range a {
		dot += float64(a[k] * b[k])
		na += float64(a[k] * a[k])
		nb += float64(b[k] * b[k])
	}
	return dot / math.Sqrt(na*nb)
}

// embedder gives each query a fixed vector: the stand-in for a model that knows two phrasings mean
// the same thing.
type embedder struct {
	vectors map[string][]float32
	err     error
}

func (e embedder) EmbedQuery(_ context.Context, text string) ([]float32, error) {
	return e.vectors[text], e.err
}

func post(slug, owner, title string, visibility entity.Visibility) entity.Blog {
	return entity.Blog{Slug: slug, OwnerID: owner, Title: title, Content: title, Visibility: visibility, CreatedAt: time.Now()}
}

func fixture(embedErr, indexErr error) (*BlogRepository, *blogs) {
	inner := &blogs{posts: map[string]entity.Blog{
		"cloud-run":  post("cloud-run", "ed", "Shipping to Cloud Run", entity.VisibilityPublic),
		"sourdough":  post("sourdough", "ed", "Baking sourdough", entity.VisibilityPublic),
		"k8s-drafts": post("k8s-drafts", "ed", "Kubernetes notes", entity.VisibilityPrivate),
	}}
	idx := index{err: indexErr, vectors: map[string][]float32{
		"cloud-run":  {0.9, 0.1, 0},
		"sourdough":  {0, 0.1, 1},
		"k8s-drafts": {1, 0, 0},
	}}
	emb := embedder{err: embedErr, vectors: map[string][]float32{"deploying containers": {1, 0.05, 0}}}
	return NewBlogRepository(inner, idx, emb, slog.New(slog.NewTextHandler(io.Discard, nil))), inner
}

func slugs(posts []entity.Blog) []string {
	out := make([]string, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.Slug)
	}
	return out
}

// A paraphrase finds the post by meaning, with no word in common, and relevance is the order.
func TestSearchFindsByMeaning(t *testing.T) {
	repo, inner := fixture(nil, nil)
	got, hasMore, err := repo.List(context.Background(), "", repository.ListParams{Query: "deploying containers", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"cloud-run", "sourdough"}; !slices.Equal(slugs(got), want) || hasMore {
		t.Errorf("got %v hasMore=%v, want %v hasMore=false", slugs(got), hasMore, want)
	}
	if inner.listed {
		t.Error("answered by the substring scan rather than by meaning")
	}
}

// The nearest neighbour is private: the index ranks it first, but only its owner is shown it.
func TestSearchKeepsReadRules(t *testing.T) {
	repo, _ := fixture(nil, nil)
	params := repository.ListParams{Query: "deploying containers", Limit: 1}

	anonymous, _, _ := repo.List(context.Background(), "", params)
	if slices.Contains(slugs(anonymous), "k8s-drafts") {
		t.Errorf("anonymous search returned a private post: %v", slugs(anonymous))
	}
	owner, _, _ := repo.List(context.Background(), "ed", params)
	if want := []string{"k8s-drafts"}; !slices.Equal(slugs(owner), want) {
		t.Errorf("owner got %v, want %v", slugs(owner), want)
	}
}

// With no model to ask, or no index to read, a search is the substring scan it used to be.
func TestSearchFallsBackToSubstringScan(t *testing.T) {
	for name, repo := range map[string]*BlogRepository{
		"embedding fails": must(fixture(errors.New("model down"), nil)),
		"index fails":     must(fixture(nil, errors.New("index down"))),
	} {
		got, _, err := repo.List(context.Background(), "", repository.ListParams{Query: "sourdough"})
		if err != nil || !slices.Equal(slugs(got), []string{"sourdough"}) {
			t.Errorf("%s: got %v, %v; want the substring match", name, slugs(got), err)
		}
	}
}

func must(repo *BlogRepository, _ *blogs) *BlogRepository { return repo }

// The feed without q never touches the index.
func TestFeedWithoutQueryIsUnchanged(t *testing.T) {
	repo, inner := fixture(nil, nil)
	if _, _, err := repo.List(context.Background(), "", repository.ListParams{}); err != nil || !inner.listed {
		t.Errorf("feed was not passed through: listed=%v err=%v", inner.listed, err)
	}
}

// A search is one page, so a StartAfter can only be continuing a fallback page, and is sent there.
func TestSearchContinuesFallbackPage(t *testing.T) {
	repo, inner := fixture(nil, nil)
	if _, _, err := repo.List(context.Background(), "", repository.ListParams{Query: "sourdough", StartAfter: time.Now()}); err != nil || !inner.listed {
		t.Errorf("continuation was not passed to the substring scan: listed=%v err=%v", inner.listed, err)
	}
}
