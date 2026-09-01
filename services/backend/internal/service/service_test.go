package service

import (
	"context"
	"encoding/json"
	"fmt"
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
// under their slug alone and refuses to overwrite an occupied one, so the globally unique slugs a
// handler derives from titles are exercised here rather than assumed.
type fakeBlogRepository struct {
	blogs map[string]entity.Blog
	// beforeCreate lets a test fail a write the in-memory state would otherwise allow, which is
	// the only way to provoke a collision against a slug that carries a random suffix.
	beforeCreate func(entity.Blog) error
}

func newFakeBlogRepository() *fakeBlogRepository {
	return &fakeBlogRepository{blogs: make(map[string]entity.Blog)}
}

// seed stores posts as a real write would, at the slug that names them - which, as in the
// Firestore repository, is the whole of the key, so two authors cannot both hold one slug.
func (r *fakeBlogRepository) seed(blogs ...entity.Blog) {
	for _, blog := range blogs {
		r.blogs[blog.Slug] = blog
	}
}

// stored returns the post held at a slug, for a test asserting on what a handler wrote.
func (r *fakeBlogRepository) stored(slug string) (entity.Blog, bool) {
	blog, ok := r.blogs[slug]
	return blog, ok
}

func (r *fakeBlogRepository) Get(_ context.Context, slug string) (entity.Blog, error) {
	blog, ok := r.stored(slug)
	if !ok || blog.IsDeleted() {
		return entity.Blog{}, repository.ErrNotFound
	}
	return blog, nil
}

func (r *fakeBlogRepository) List(_ context.Context, uid string) ([]entity.Blog, error) {
	visible := make([]entity.Blog, 0, len(r.blogs))
	for _, blog := range r.blogs {
		if !blog.IsDeleted() && blog.CanBeReadBy(uid) {
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
	if _, taken := r.stored(blog.Slug); taken {
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

// Delete mirrors the Firestore repository: the document stays, only DeletedAt is stamped.
func (r *fakeBlogRepository) Delete(_ context.Context, slug string) error {
	blog, ok := r.stored(slug)
	if !ok || blog.IsDeleted() {
		return repository.ErrNotFound
	}
	now := time.Now().UTC()
	blog.DeletedAt = &now
	r.seed(blog)
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

// fakeChatRepository is an in-memory repository.ChatRepository. Like the real one it keys a chat
// by the slug of the post it is about and keeps the owner of an existing chat rather than taking
// the caller's, so ownership cannot be reassigned by appending to one.
type fakeChatRepository struct {
	chats map[string]entity.Chat
	// appendErr fails a write the in-memory state would otherwise allow.
	appendErr error
}

func newFakeChatRepository() *fakeChatRepository {
	return &fakeChatRepository{chats: make(map[string]entity.Chat)}
}

func (r *fakeChatRepository) seed(chat entity.Chat) {
	r.chats[chat.BlogSlug] = chat
}

func (r *fakeChatRepository) Get(_ context.Context, blogSlug string) (entity.Chat, error) {
	chat, ok := r.chats[blogSlug]
	if !ok {
		return entity.Chat{}, repository.ErrNotFound
	}
	return chat, nil
}

func (r *fakeChatRepository) Append(_ context.Context, blogSlug, ownerID string, messages ...entity.ChatMessage) (entity.Chat, error) {
	if r.appendErr != nil {
		return entity.Chat{}, r.appendErr
	}

	chat, ok := r.chats[blogSlug]
	if !ok {
		chat = entity.Chat{BlogSlug: blogSlug, OwnerID: ownerID, CreatedAt: time.Now().UTC()}
	}
	if err := chat.Validate(); err != nil {
		return entity.Chat{}, err
	}
	if err := chat.Append(messages...); err != nil {
		return entity.Chat{}, err
	}
	chat.UpdatedAt = time.Now().UTC()

	r.seed(chat)
	return chat, nil
}

func (r *fakeChatRepository) Delete(_ context.Context, blogSlug string) error {
	delete(r.chats, blogSlug)
	return nil
}

// fakeCommentRepository is an in-memory repository.CommentRepository. Like the real one it keeps
// each thread beneath the post it is on, assigns the id itself rather than taking one from the
// caller, and hands a thread back oldest first.
type fakeCommentRepository struct {
	threads map[string][]entity.Comment
	// created counts assigned ids, so each comment gets a distinct one as a real write would.
	created int
	// createErr fails a write the in-memory state would otherwise allow.
	createErr error
}

func newFakeCommentRepository() *fakeCommentRepository {
	return &fakeCommentRepository{threads: make(map[string][]entity.Comment)}
}

// seed stores comments as a real write would, in the thread of the post they name.
func (r *fakeCommentRepository) seed(comments ...entity.Comment) {
	for _, comment := range comments {
		r.threads[comment.BlogSlug] = append(r.threads[comment.BlogSlug], comment)
	}
}

func (r *fakeCommentRepository) List(_ context.Context, blogSlug string) ([]entity.Comment, error) {
	return slices.Clone(r.threads[blogSlug]), nil
}

func (r *fakeCommentRepository) Get(_ context.Context, blogSlug, id string) (entity.Comment, error) {
	for _, comment := range r.threads[blogSlug] {
		if comment.ID == id {
			return comment, nil
		}
	}
	return entity.Comment{}, repository.ErrNotFound
}

func (r *fakeCommentRepository) Create(_ context.Context, comment entity.Comment) (entity.Comment, error) {
	if err := comment.Validate(); err != nil {
		return entity.Comment{}, err
	}
	if r.createErr != nil {
		return entity.Comment{}, r.createErr
	}

	r.created++
	// Shaped like the ids Firestore assigns - letters and digits only - since that is what
	// entity.Comment.SetID admits when one comes back in a URL.
	comment.ID = fmt.Sprintf("cmt%d", r.created)
	comment.CreatedAt = time.Now().UTC()

	r.seed(comment)
	return comment, nil
}

func (r *fakeCommentRepository) Delete(_ context.Context, blogSlug, id string) error {
	r.threads[blogSlug] = slices.DeleteFunc(r.threads[blogSlug], func(comment entity.Comment) bool {
		return comment.ID == id
	})
	return nil
}

// fakeAssistant is an in-memory repository.Assistant. reply stands in for the model, so a test
// decides what it says and what it edits without a provider anywhere in the picture.
type fakeAssistant struct {
	reply func(repository.AssistantRequest) (repository.AssistantReply, error)
	// requests records what the service asked, for a test asserting on the draft or the history it
	// was handed.
	requests []repository.AssistantRequest
}

func (a *fakeAssistant) Reply(_ context.Context, req repository.AssistantRequest) (repository.AssistantReply, error) {
	a.requests = append(a.requests, req)
	if a.reply == nil {
		return repository.AssistantReply{Text: "ok", Draft: req.Draft}, nil
	}
	return a.reply(req)
}

// fakeVerifier is an in-memory repository.TokenVerifier. email and emailVerified describe the
// address the provider vouches for, for a test reaching a route the assistant allowlist guards
// through the real middleware; the zero value is an address nobody verified, which no allowlist
// matches however it is spelled.
type fakeVerifier struct {
	uid           string
	email         string
	emailVerified bool
	err           error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (entity.Caller, error) {
	if f.err != nil {
		return entity.Caller{}, f.err
	}
	email := f.email
	if email == "" {
		email = "user@example.com"
	}
	return entity.Caller{UID: f.uid, Email: email, Name: "User", EmailVerified: f.emailVerified}, nil
}

// newTestService builds a Service over the given fakes, filling in whichever are not needed. The
// assistant allowlist is empty, so a test that does not opt in is testing a deployment where the
// assistant is off - which is every deployment but the one account it is enabled for.
func newTestService(blogs repository.BlogRepository, users repository.UserRepository) *Service {
	return newCommentService(blogs, users, nil)
}

// newCommentService builds a Service over a comment repository, for the routes that have one. The
// assistant is off here too: commenting is the readers' half of a post and has nothing to do with
// it.
func newCommentService(
	blogs repository.BlogRepository,
	users repository.UserRepository,
	comments repository.CommentRepository,
) *Service {
	return newFullService(blogs, users, nil, comments, nil, nil)
}

// newAssistantService builds a Service with the assistant enabled for the named addresses.
func newAssistantService(
	blogs repository.BlogRepository,
	users repository.UserRepository,
	chats repository.ChatRepository,
	assistant repository.Assistant,
	allowed []string,
) *Service {
	return newFullService(blogs, users, chats, nil, assistant, allowed)
}

// newFullService is what the helpers above narrow: it fills in whichever fakes a test did not
// supply, so each of them names only the repositories its routes actually touch.
func newFullService(
	blogs repository.BlogRepository,
	users repository.UserRepository,
	chats repository.ChatRepository,
	comments repository.CommentRepository,
	assistant repository.Assistant,
	allowed []string,
) *Service {
	if blogs == nil {
		blogs = newFakeBlogRepository()
	}
	if users == nil {
		users = newFakeUserRepository()
	}
	if chats == nil {
		chats = newFakeChatRepository()
	}
	if comments == nil {
		comments = newFakeCommentRepository()
	}
	if assistant == nil {
		assistant = &fakeAssistant{}
	}

	return New(
		Config{
			Environment:        "test",
			Commit:             "abc123",
			AssistantAllowlist: entity.NewAssistantAllowlist(allowed),
		},
		blogs, users, chats, comments, fakeVerifier{uid: "caller"}, assistant,
	)
}

func withUID(req *http.Request, uid string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), callerContextKey, entity.Caller{UID: uid}))
}

// withVerifiedCaller carries an address the provider vouched for, which is what the assistant
// allowlist is keyed on. withUID's caller deliberately has none: most routes do not care, and the
// ones that do must not be satisfied by an address nobody verified.
func withVerifiedCaller(req *http.Request, uid, email string) *http.Request {
	caller := entity.Caller{UID: uid, Email: email, EmailVerified: true}
	return req.WithContext(context.WithValue(req.Context(), callerContextKey, caller))
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
