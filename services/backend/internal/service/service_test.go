package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// fakeBlogRepository is an in-memory repository.BlogRepository.
type fakeBlogRepository struct {
	blogs  map[string]entity.Blog
	nextID int
}

func newFakeBlogRepository() *fakeBlogRepository {
	return &fakeBlogRepository{blogs: make(map[string]entity.Blog)}
}

func (r *fakeBlogRepository) Get(_ context.Context, id string) (entity.Blog, error) {
	blog, ok := r.blogs[id]
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
	r.nextID++
	blog.ID = fmt.Sprintf("blog-%d", r.nextID)
	r.blogs[blog.ID] = blog
	return blog, nil
}

func (r *fakeBlogRepository) Update(_ context.Context, blog entity.Blog) (entity.Blog, error) {
	if err := blog.Validate(); err != nil {
		return entity.Blog{}, err
	}
	r.blogs[blog.ID] = blog
	return blog, nil
}

func (r *fakeBlogRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.blogs[id]; !ok {
		return repository.ErrNotFound
	}
	delete(r.blogs, id)
	return nil
}

// fakeUserRepository is an in-memory repository.UserRepository. gets counts lookups so a test can
// assert a handler did not reach the datastore.
type fakeUserRepository struct {
	users map[string]entity.User
	gets  int
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{users: make(map[string]entity.User)}
}

func (r *fakeUserRepository) Get(_ context.Context, id string) (entity.User, error) {
	r.gets++
	user, ok := r.users[id]
	if !ok {
		return entity.User{}, repository.ErrNotFound
	}
	return user, nil
}

func (r *fakeUserRepository) Put(_ context.Context, user entity.User) (entity.User, error) {
	if err := user.Validate(); err != nil {
		return entity.User{}, err
	}
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	r.users[user.ID] = user
	return user, nil
}

func (r *fakeUserRepository) Delete(_ context.Context, id string) error {
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
