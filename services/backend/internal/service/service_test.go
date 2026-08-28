package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// fakeBlogRepository is an in-memory repository.BlogRepository. Like the real one it stores posts
// under their owner and slug together and refuses to overwrite an occupied pair, so the per-author
// slugs a handler derives from titles are exercised here rather than assumed.
type fakeBlogRepository struct {
	blogs map[string]entity.Blog
	// beforeCreate lets a test fail a write the in-memory state would otherwise allow, which is
	// the only way to provoke a collision against a slug that carries a random suffix.
	beforeCreate func(entity.Blog) error
}

func newFakeBlogRepository() *fakeBlogRepository {
	return &fakeBlogRepository{blogs: make(map[string]entity.Blog)}
}

// fakeBlogKey mirrors the Firestore repository's document key: a slug belongs to one author, so
// the same slug under two owners is two posts.
func fakeBlogKey(ownerID, slug string) string {
	return ownerID + "/" + slug
}

// seed stores posts as a real write would, at the key their owner and slug name.
func (r *fakeBlogRepository) seed(blogs ...entity.Blog) {
	for _, blog := range blogs {
		r.blogs[fakeBlogKey(blog.OwnerID, blog.Slug)] = blog
	}
}

// stored returns the post held at a key, for a test asserting on what a handler wrote.
func (r *fakeBlogRepository) stored(ownerID, slug string) (entity.Blog, bool) {
	blog, ok := r.blogs[fakeBlogKey(ownerID, slug)]
	return blog, ok
}

func (r *fakeBlogRepository) Get(_ context.Context, ownerID, slug string) (entity.Blog, error) {
	blog, ok := r.stored(ownerID, slug)
	if !ok {
		return entity.Blog{}, repository.ErrNotFound
	}
	return blog, nil
}

func (r *fakeBlogRepository) List(_ context.Context, uid string) ([]entity.Blog, error) {
	visible := make([]entity.Blog, 0, len(r.blogs))
	for _, blog := range r.blogs {
		if blog.CanBeReadBy(uid) {
			visible = append(visible, blog)
		}
	}
	slices.SortFunc(visible, func(a, b entity.Blog) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return visible, nil
}

func (r *fakeBlogRepository) Create(_ context.Context, blog entity.Blog) (entity.Blog, error) {
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}
	if r.beforeCreate != nil {
		if err := r.beforeCreate(blog); err != nil {
			return entity.Blog{}, err
		}
	}
	if _, taken := r.stored(blog.OwnerID, blog.Slug); taken {
		return entity.Blog{}, repository.ErrSlugTaken
	}

	now := time.Now().UTC()
	blog.CreatedAt = now
	blog.UpdatedAt = now

	r.seed(blog)
	return blog, nil
}

func (r *fakeBlogRepository) Update(_ context.Context, blog entity.Blog) (entity.Blog, error) {
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}
	r.seed(blog)
	return blog, nil
}

func (r *fakeBlogRepository) Delete(_ context.Context, ownerID, slug string) error {
	if _, ok := r.stored(ownerID, slug); !ok {
		return repository.ErrNotFound
	}
	delete(r.blogs, fakeBlogKey(ownerID, slug))
	return nil
}

// fakeUserRepository is an in-memory repository.UserRepository. usernames mirrors the reservation
// collection the Firestore implementation keeps, so uniqueness is enforced here too rather than
// assumed. gets counts lookups so a test can assert a handler did not reach the datastore.
type fakeUserRepository struct {
	users     map[string]entity.User
	usernames map[string]string
	gets      int
	// beforePut lets a test fail a write the in-memory state would otherwise allow, which is the
	// only way to provoke a username collision when the name being written is generated at random.
	beforePut func(entity.User) error
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:     make(map[string]entity.User),
		usernames: make(map[string]string),
	}
}

// seed stores a profile and its reservation together, as a real write would.
func (r *fakeUserRepository) seed(user entity.User) {
	r.users[user.ID] = user
	r.usernames[user.UsernameKey()] = user.ID
}

func (r *fakeUserRepository) Get(_ context.Context, id string) (entity.User, error) {
	r.gets++
	user, ok := r.users[id]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (r *fakeUserRepository) GetByUsername(_ context.Context, username string) (entity.User, error) {
	r.gets++
	id, ok := r.usernames[strings.ToLower(username)]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}
	user, ok := r.users[id]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (r *fakeUserRepository) Put(_ context.Context, user entity.User) (entity.User, error) {
	user, err := user.Normalized()
	if err != nil {
		return entity.User{}, err
	}
	if r.beforePut != nil {
		if err := r.beforePut(user); err != nil {
			return entity.User{}, err
		}
	}
	key := user.UsernameKey()
	if owner, claimed := r.usernames[key]; claimed && owner != user.ID {
		return entity.User{}, repository.ErrUsernameTaken
	}

	now := time.Now().UTC()
	if previous, ok := r.users[user.ID]; ok {
		user.CreatedAt = previous.CreatedAt
		delete(r.usernames, previous.UsernameKey())
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	r.users[user.ID] = user
	r.usernames[key] = user.ID
	return user, nil
}

func (r *fakeUserRepository) Delete(_ context.Context, id string) error {
	if user, ok := r.users[id]; ok {
		delete(r.usernames, user.UsernameKey())
	}
	delete(r.users, id)
	return nil
}

// fakeVerifier is an in-memory repository.TokenVerifier.
type fakeVerifier struct {
	uid string
	err error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (entity.Caller, error) {
	if f.err != nil {
		return entity.Caller{}, f.err
	}
	return entity.Caller{UID: f.uid, Email: "user@example.com", Name: "User"}, nil
}

// newTestService builds a Service over the given fakes, filling in whichever are not needed.
func newTestService(blogs repository.BlogRepository, users repository.UserRepository) *Service {
	if blogs == nil {
		blogs = newFakeBlogRepository()
	}
	if users == nil {
		users = newFakeUserRepository()
	}
	return New(Config{Environment: "test", Commit: "abc123"}, blogs, users, fakeVerifier{uid: "caller"})
}

func withUID(req *http.Request, uid string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), callerContextKey, entity.Caller{UID: uid}))
}

// decodeAPIError asserts the response carries a JSON error body rather than plain text.
func decodeAPIError(t *testing.T, rec *httptest.ResponseRecorder) apiError {
	t.Helper()

	if ct := rec.Result().Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	var body apiError
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error == "" {
		t.Error("error body has an empty message")
	}
	return body
}
