package firestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	fs "cloud.google.com/go/firestore"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// These tests run the adapters against the Firestore emulator, so the queries, transactions and
// collection-group reads are exercised by Firestore itself rather than by a fake's idea of it. They
// skip when FIRESTORE_EMULATOR_HOST is unset, which keeps `go test ./...` passing without one; CI
// starts one (see the `test` job in .github/workflows/pull-request.yaml). The emulator does not
// enforce composite indexes, so a missing index is still only caught in staging.

// newClient connects to the emulator under a project of the test's own: the emulator keeps each
// project's data apart, so no test sees another's documents and none needs cleaning up after.
func newClient(t *testing.T) *fs.Client {
	t.Helper()
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is unset; see services/backend/README.md to run these")
	}
	client, err := fs.NewClient(context.Background(), fmt.Sprintf("test-%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("firestore client: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func createBlog(t *testing.T, blogs *BlogRepository, blog entity.Blog) entity.Blog {
	t.Helper()
	if blog.Title == "" {
		blog.Title = blog.Slug
	}
	if blog.Content == "" {
		blog.Content = "Body of " + blog.Slug
	}
	if blog.Visibility == "" {
		blog.Visibility = entity.VisibilityPublic
	}
	created, err := blogs.Create(context.Background(), blog)
	if err != nil {
		t.Fatalf("Create(%s): %v", blog.Slug, err)
	}
	return created
}

func slugsOf(blogs []entity.Blog) []string {
	slugs := make([]string, 0, len(blogs))
	for _, blog := range blogs {
		slugs = append(slugs, blog.Slug)
	}
	return slugs
}

func TestEmulatorBlog_CreateGetAndSlugTaken(t *testing.T) {
	ctx := context.Background()
	blogs := NewBlogRepository(newClient(t))

	created := createBlog(t, blogs, entity.Blog{Slug: "hello", OwnerID: "ann", Tags: []string{"go"}})

	got, err := blogs.Get(ctx, "hello")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != created.Title || got.OwnerID != "ann" || !slices.Equal(got.Tags, []string{"go"}) {
		t.Errorf("Get = %+v, want %+v", got, created)
	}
	if _, err := blogs.Create(ctx, entity.Blog{Slug: "hello", OwnerID: "bob", Title: "t", Content: "c", Visibility: entity.VisibilityPublic}); !errors.Is(err, repository.ErrSlugTaken) {
		t.Errorf("second Create = %v, want ErrSlugTaken", err)
	}
	if _, err := blogs.Get(ctx, "missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get(missing) = %v, want ErrNotFound", err)
	}
}

// The general feed's OR query, the profile feed's owner filter and the tag filter each reach
// Firestore as a different query; all three must still admit only what CanBeReadBy allows.
func TestEmulatorBlog_ListFilters(t *testing.T) {
	ctx := context.Background()
	blogs := NewBlogRepository(newClient(t))

	createBlog(t, blogs, entity.Blog{Slug: "ann-public", OwnerID: "ann", Tags: []string{"go"}})
	createBlog(t, blogs, entity.Blog{Slug: "ann-private", OwnerID: "ann", Visibility: entity.VisibilityPrivate, Tags: []string{"go"}})
	createBlog(t, blogs, entity.Blog{Slug: "ann-shared", OwnerID: "ann", Visibility: entity.VisibilityPrivate, AllowedUserIDs: []string{"bob"}, Tags: []string{"go"}})
	createBlog(t, blogs, entity.Blog{Slug: "bob-public", OwnerID: "bob", Tags: []string{"web"}, Content: "Shipping to Cloud Run"})
	createBlog(t, blogs, entity.Blog{Slug: "ann-deleted", OwnerID: "ann", Tags: []string{"go"}})
	if err := blogs.Delete(ctx, "ann-deleted"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	for _, tt := range []struct {
		name   string
		uid    string
		params repository.ListParams
		want   []string
	}{
		{"anonymous feed", "", repository.ListParams{}, []string{"bob-public", "ann-public"}},
		{"owner feed", "ann", repository.ListParams{}, []string{"bob-public", "ann-shared", "ann-private", "ann-public"}},
		{"whitelisted feed", "bob", repository.ListParams{}, []string{"bob-public", "ann-shared", "ann-public"}},
		{"author, anonymous", "", repository.ListParams{OwnerUID: "ann"}, []string{"ann-public"}},
		{"author, whitelisted", "bob", repository.ListParams{OwnerUID: "ann"}, []string{"ann-shared", "ann-public"}},
		{"tag, anonymous", "", repository.ListParams{Tag: "go"}, []string{"ann-public"}},
		{"tag, owner", "ann", repository.ListParams{Tag: "go"}, []string{"ann-shared", "ann-private", "ann-public"}},
		{"tag and author", "bob", repository.ListParams{Tag: "go", OwnerUID: "ann"}, []string{"ann-shared", "ann-public"}},
		{"tag nobody uses", "ann", repository.ListParams{Tag: "rust"}, []string{}},
		{"query", "", repository.ListParams{Query: "cloud run"}, []string{"bob-public"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, hasMore, err := blogs.List(ctx, tt.uid, tt.params)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if !slices.Equal(slugsOf(got), tt.want) || hasMore {
				t.Errorf("List = %v (hasMore %v), want %v", slugsOf(got), hasMore, tt.want)
			}
		})
	}
}

func TestEmulatorBlog_ListPages(t *testing.T) {
	ctx := context.Background()
	blogs := NewBlogRepository(newClient(t))
	for _, slug := range []string{"one", "two", "three"} {
		createBlog(t, blogs, entity.Blog{Slug: slug, OwnerID: "ann"})
	}

	first, hasMore, err := blogs.List(ctx, "", repository.ListParams{Limit: 2})
	if err != nil || !slices.Equal(slugsOf(first), []string{"three", "two"}) || !hasMore {
		t.Fatalf("first page = %v (hasMore %v, err %v), want [three two] with more", slugsOf(first), hasMore, err)
	}
	second, hasMore, err := blogs.List(ctx, "", repository.ListParams{Limit: 2, StartAfter: first[1].CreatedAt})
	if err != nil || !slices.Equal(slugsOf(second), []string{"one"}) || hasMore {
		t.Fatalf("second page = %v (hasMore %v, err %v), want [one] and no more", slugsOf(second), hasMore, err)
	}
}

// The stale-write refusal (#252): an update carrying an UpdatedAt other than the stored one is
// refused inside the transaction rather than overwriting the newer write.
func TestEmulatorBlog_Update(t *testing.T) {
	ctx := context.Background()
	blogs := NewBlogRepository(newClient(t))
	created := createBlog(t, blogs, entity.Blog{Slug: "post", OwnerID: "ann"})

	edited := created
	edited.Title = "Edited"
	updated, err := blogs.Update(ctx, edited)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, _ := blogs.Get(ctx, "post"); got.Title != "Edited" {
		t.Errorf("stored title = %q, want Edited", got.Title)
	}

	stale := created
	stale.Title = "Stale"
	if _, err := blogs.Update(ctx, stale); !errors.Is(err, repository.ErrPostChanged) {
		t.Errorf("stale Update = %v, want ErrPostChanged", err)
	}
	if got, _ := blogs.Get(ctx, "post"); got.Title != "Edited" {
		t.Errorf("stored title after stale write = %q, want Edited", got.Title)
	}

	if _, err := blogs.Update(ctx, updated); err != nil {
		t.Errorf("Update from the latest read = %v, want nil", err)
	}

	missing := created
	missing.Slug = "missing"
	if _, err := blogs.Update(ctx, missing); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Update(missing) = %v, want ErrNotFound", err)
	}
}

func TestEmulatorBlog_DeleteOwnedSlugsPurge(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	blogs := NewBlogRepository(client)
	comments := NewCommentRepository(client)
	reactions := NewReactionRepository(client)

	createBlog(t, blogs, entity.Blog{Slug: "kept", OwnerID: "ann"})
	createBlog(t, blogs, entity.Blog{Slug: "gone", OwnerID: "ann"})
	createBlog(t, blogs, entity.Blog{Slug: "other", OwnerID: "bob"})

	if err := blogs.Delete(ctx, "gone"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := blogs.Get(ctx, "gone"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get(soft-deleted) = %v, want ErrNotFound", err)
	}
	if _, err := blogs.Update(ctx, entity.Blog{Slug: "gone", OwnerID: "ann", Title: "t", Content: "c", Visibility: entity.VisibilityPublic}); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Update(soft-deleted) = %v, want ErrNotFound", err)
	}
	if err := blogs.Delete(ctx, "missing"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Delete(missing) = %v, want ErrNotFound", err)
	}

	owned, err := blogs.Owned(ctx, "ann")
	slices.Sort(owned)
	if err != nil || !slices.Equal(owned, []string{"gone", "kept"}) {
		t.Errorf("Owned = %v, %v; want [gone kept]", owned, err)
	}
	all, err := blogs.Slugs(ctx)
	slices.Sort(all)
	if err != nil || !slices.Equal(all, []string{"gone", "kept", "other"}) {
		t.Errorf("Slugs = %v, %v; want [gone kept other]", all, err)
	}

	if _, err := comments.Create(ctx, entity.Comment{BlogSlug: "kept", AuthorID: "bob", Body: "hi"}); err != nil {
		t.Fatalf("comment Create: %v", err)
	}
	if _, err := reactions.Add(ctx, entity.PostReaction("kept"), "bob", entity.AllowedEmojis[0]); err != nil {
		t.Fatalf("reaction Add: %v", err)
	}
	if err := blogs.Purge(ctx, "kept"); err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if left, _ := comments.List(ctx, "kept"); len(left) != 0 {
		t.Errorf("comments after Purge = %v, want none", left)
	}
	if left, _ := reactions.List(ctx, "kept"); len(left) != 0 {
		t.Errorf("reactions after Purge = %v, want none", left)
	}
	if all, _ := blogs.Slugs(ctx); slices.Contains(all, "kept") {
		t.Errorf("Slugs after Purge = %v, want kept gone", all)
	}
}

func TestEmulatorComment(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	comments := NewCommentRepository(client)

	first, err := comments.Create(ctx, entity.Comment{BlogSlug: "post", AuthorID: "bob", Body: "first"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := comments.Create(ctx, entity.Comment{BlogSlug: "post", AuthorID: "ann", Body: "second"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := comments.Create(ctx, entity.Comment{BlogSlug: "elsewhere", AuthorID: "bob", Body: "third"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	listed, err := comments.List(ctx, "post")
	if err != nil || len(listed) != 2 || listed[0].Body != "first" || listed[1].Body != "second" {
		t.Fatalf("List = %+v, %v; want first then second", listed, err)
	}

	moderation, _ := entity.NewModeration(true, "spam", "model")
	if err := comments.SetModeration(ctx, "post", first.ID, moderation); err != nil {
		t.Fatalf("SetModeration: %v", err)
	}
	got, err := comments.Get(ctx, "post", first.ID)
	if err != nil || got.Moderation == nil || got.Moderation.Status != entity.ModerationFlagged || got.Moderation.Category != "spam" {
		t.Errorf("Get after SetModeration = %+v, %v; want flagged as spam", got, err)
	}
	if err := comments.SetModeration(ctx, "post", "missing", moderation); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("SetModeration(missing) = %v, want ErrNotFound", err)
	}

	byBob, err := comments.ListByAuthor(ctx, "bob")
	if err != nil || len(byBob) != 2 {
		t.Errorf("ListByAuthor(bob) = %+v, %v; want two across both posts", byBob, err)
	}

	if err := comments.Delete(ctx, "post", first.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := comments.Get(ctx, "post", first.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
}

func TestEmulatorReaction(t *testing.T) {
	ctx := context.Background()
	reactions := NewReactionRepository(newClient(t))
	post, comment := entity.PostReaction("post"), entity.CommentReaction("post", "abc")
	like, other := entity.AllowedEmojis[0], entity.AllowedEmojis[1]

	if _, err := reactions.Add(ctx, post, "bob", like); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Addressed rather than toggled, so a retried add changes nothing.
	got, err := reactions.Add(ctx, post, "bob", like)
	if err != nil || !slices.Equal(got.Emojis, []string{like}) {
		t.Errorf("repeated Add = %+v, %v; want just %s", got, err, like)
	}
	if _, err := reactions.Add(ctx, post, "bob", other); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := reactions.Add(ctx, comment, "bob", like); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := reactions.Add(ctx, comment, "ann", like); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := reactions.Add(ctx, entity.PostReaction("elsewhere"), "bob", like); err != nil {
		t.Fatalf("Add: %v", err)
	}

	listed, err := reactions.List(ctx, "post")
	if err != nil || len(listed) != 3 {
		t.Fatalf("List = %+v, %v; want three", listed, err)
	}

	got, err = reactions.Remove(ctx, post, "bob", like)
	if err != nil || !slices.Equal(got.Emojis, []string{other}) {
		t.Errorf("Remove = %+v, %v; want just %s", got, err, other)
	}
	// Removing the last emoji deletes the document rather than storing an empty one.
	if _, err := reactions.Remove(ctx, post, "bob", other); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if listed, _ := reactions.List(ctx, "post"); len(listed) != 2 {
		t.Errorf("List after emptying = %+v, want the two comment reactions", listed)
	}

	if err := reactions.DeleteTarget(ctx, comment); err != nil {
		t.Fatalf("DeleteTarget: %v", err)
	}
	if listed, _ := reactions.List(ctx, "post"); len(listed) != 0 {
		t.Errorf("List after DeleteTarget = %+v, want none", listed)
	}

	if err := reactions.DeleteByUser(ctx, "bob"); err != nil {
		t.Fatalf("DeleteByUser: %v", err)
	}
	if listed, _ := reactions.List(ctx, "elsewhere"); len(listed) != 0 {
		t.Errorf("List after DeleteByUser = %+v, want none", listed)
	}
}

func TestEmulatorUser(t *testing.T) {
	ctx := context.Background()
	users := NewUserRepository(newClient(t))

	if _, err := users.Create(ctx, entity.User{ID: "ann", Username: "Ann"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := users.Create(ctx, entity.User{ID: "ann", Username: "other"}); !errors.Is(err, repository.ErrUserExists) {
		t.Errorf("second Create = %v, want ErrUserExists", err)
	}
	if _, err := users.Create(ctx, entity.User{ID: "bob", Username: "ann"}); !errors.Is(err, repository.ErrUsernameTaken) {
		t.Errorf("Create with a held username = %v, want ErrUsernameTaken", err)
	}

	got, err := users.GetByUsername(ctx, "ANN")
	if err != nil || got.ID != "ann" {
		t.Fatalf("GetByUsername = %+v, %v; want ann", got, err)
	}

	// A rename releases the old name for anybody to claim.
	if _, err := users.Put(ctx, entity.User{ID: "ann", Username: "annie", Bio: "hi"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := users.GetByUsername(ctx, "ann"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetByUsername(old name) = %v, want ErrNotFound", err)
	}
	if _, err := users.Put(ctx, entity.User{ID: "bob", Username: "ann"}); err != nil {
		t.Errorf("Put claiming a released name = %v, want nil", err)
	}
	if got, err := users.Get(ctx, "ann"); err != nil || got.Username != "annie" || got.Bio != "hi" || got.CreatedAt.IsZero() {
		t.Errorf("Get = %+v, %v; want annie with a bio and a creation time", got, err)
	}

	if err := users.Delete(ctx, "ann"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := users.Get(ctx, "ann"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
	if _, err := users.GetByUsername(ctx, "annie"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("GetByUsername after Delete = %v, want ErrNotFound", err)
	}
	if err := users.Delete(ctx, "ann"); err != nil {
		t.Errorf("repeated Delete = %v, want nil", err)
	}
}

func TestEmulatorChat(t *testing.T) {
	ctx := context.Background()
	chats := NewChatRepository(newClient(t))

	question, _ := entity.NewChatMessage(entity.ChatRoleUser, "Tighten the intro?")
	answer, _ := entity.NewChatMessage(entity.ChatRoleAssistant, "Done.", entity.ChatEdit{Tool: "edit", Summary: "intro"})

	if _, err := chats.Append(ctx, "post", "ann", question); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if _, err := chats.Append(ctx, "post", "ann", answer); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := chats.Get(ctx, "post")
	if err != nil || len(got.Messages) != 2 || got.Messages[1].Edits[0].Summary != "intro" || got.OwnerID != "ann" {
		t.Fatalf("Get = %+v, %v; want both messages", got, err)
	}

	if err := chats.Delete(ctx, "post"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := chats.Get(ctx, "post"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
}

func TestEmulatorEmbedding(t *testing.T) {
	ctx := context.Background()
	client := newClient(t)
	blogs := NewBlogRepository(client)
	embeddings := NewEmbeddingRepository(client)

	post := createBlog(t, blogs, entity.Blog{Slug: "post", OwnerID: "ann"})
	near := createBlog(t, blogs, entity.Blog{Slug: "near", OwnerID: "ann"})
	far := createBlog(t, blogs, entity.Blog{Slug: "far", OwnerID: "ann"})

	put := func(blog entity.Blog, vector []float32) {
		t.Helper()
		err := embeddings.Put(ctx, entity.Embedding{
			Slug: blog.Slug, OwnerID: blog.OwnerID, Vector: vector,
			ContentHash: entity.ContentHash(entity.EmbeddingText(blog)), Model: "model",
		})
		if err != nil {
			t.Fatalf("Put(%s): %v", blog.Slug, err)
		}
	}
	put(post, []float32{1, 0})
	put(near, []float32{0.9, 0.1})
	put(far, []float32{0, 1})

	// An embedding of text the post no longer holds is dropped rather than stored.
	if err := embeddings.Put(ctx, entity.Embedding{Slug: "post", Vector: []float32{0, 1}, ContentHash: "stale"}); err != nil {
		t.Fatalf("stale Put: %v", err)
	}
	got, err := embeddings.Get(ctx, "post")
	if err != nil || !slices.Equal(got.Vector, []float32{1, 0}) || got.Model != "model" {
		t.Errorf("Get = %+v, %v; want the first vector kept", got, err)
	}

	nearest, err := embeddings.Nearest(ctx, []float32{1, 0}, 2)
	if err != nil || !slices.Equal(nearest, []string{"post", "near"}) {
		t.Errorf("Nearest = %v, %v; want [post near]", nearest, err)
	}

	if err := embeddings.Delete(ctx, "post"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := embeddings.Get(ctx, "post"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
}
