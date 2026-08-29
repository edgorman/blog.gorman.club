package entity

import (
	"errors"
	"strings"
	"testing"
)

func TestDraftOf(t *testing.T) {
	blog := Blog{Slug: "hello", OwnerID: "owner", Title: "Hello", Content: "body", Visibility: VisibilityPrivate}

	draft := DraftOf(blog)

	if draft.Title != "Hello" || draft.Content != "body" {
		t.Errorf("DraftOf = %+v, want the title and content", draft)
	}
}

func TestDraft_SetTitle(t *testing.T) {
	var draft Draft

	if err := draft.SetTitle("  Hello  "); err != nil {
		t.Fatalf("SetTitle = %v, want no error", err)
	}
	if draft.Title != "Hello" {
		t.Errorf("Title = %q, want %q", draft.Title, "Hello")
	}

	// The rule is Blog.SetTitle's, so a draft cannot hold a title the post would reject.
	err := draft.SetTitle(strings.Repeat("a", MaxTitleLength+1))

	var invalid ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("SetTitle(too long) = %v, want a ValidationError", err)
	}
	if draft.Title != "Hello" {
		t.Errorf("Title = %q, want the rejected value not to be applied", draft.Title)
	}
}

// Whitespace is significant in a post body, so unlike the title it is not trimmed.
func TestDraft_SetContent(t *testing.T) {
	var draft Draft

	if err := draft.SetContent("  spaced  "); err != nil {
		t.Fatalf("SetContent = %v, want no error", err)
	}
	if draft.Content != "  spaced  " {
		t.Errorf("Content = %q, want it untrimmed", draft.Content)
	}

	err := draft.SetContent(strings.Repeat("a", MaxContentLength+1))

	var invalid ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("SetContent(too long) = %v, want a ValidationError", err)
	}
	if draft.Content != "  spaced  " {
		t.Errorf("Content = %q, want the rejected value not to be applied", draft.Content)
	}
}

func TestDraft_ReplaceText(t *testing.T) {
	draft := Draft{Content: "the cat sat on the mat, and the cat stayed"}

	if err := draft.ReplaceText("cat", "dog"); err != nil {
		t.Fatalf("ReplaceText = %v, want no error", err)
	}
	want := "the dog sat on the mat, and the dog stayed"
	if draft.Content != want {
		t.Errorf("Content = %q, want %q", draft.Content, want)
	}

	// An empty replacement deletes the passage, which is how the model removes a sentence.
	if err := draft.ReplaceText(", and the dog stayed", ""); err != nil {
		t.Fatalf("ReplaceText(delete) = %v, want no error", err)
	}
	if draft.Content != "the dog sat on the mat" {
		t.Errorf("Content = %q, want the passage deleted", draft.Content)
	}
}

// A passage that is not there is an error rather than a silent no-op: the caller is a model
// working from a copy, and it cannot notice a replacement that did not happen.
func TestDraft_ReplaceTextMissing(t *testing.T) {
	for _, find := range []string{"", "elephant"} {
		t.Run(find, func(t *testing.T) {
			draft := Draft{Content: "the cat sat"}

			err := draft.ReplaceText(find, "dog")

			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("ReplaceText(%q) = %v, want a ValidationError", find, err)
			}
			if invalid.Field != "find" {
				t.Errorf("Field = %q, want %q", invalid.Field, "find")
			}
			if draft.Content != "the cat sat" {
				t.Errorf("Content = %q, want it unchanged", draft.Content)
			}
		})
	}
}

// The draft carries the title and body and nothing else, so writing one back cannot move a post,
// hand it to somebody else, or publish a private one.
func TestDraft_ApplyTo(t *testing.T) {
	blog := Blog{
		Slug:           "hello",
		OwnerID:        "owner",
		Title:          "Hello",
		Content:        "body",
		Visibility:     VisibilityPrivate,
		AllowedUserIDs: []string{"friend"},
	}
	draft := Draft{Title: "  Hello again  ", Content: "new body"}

	if err := draft.ApplyTo(&blog); err != nil {
		t.Fatalf("ApplyTo = %v, want no error", err)
	}

	if blog.Title != "Hello again" {
		t.Errorf("Title = %q, want the trimmed new title", blog.Title)
	}
	if blog.Content != "new body" {
		t.Errorf("Content = %q, want the new body", blog.Content)
	}
	if blog.Slug != "hello" || blog.OwnerID != "owner" {
		t.Errorf("identity changed: %+v", blog)
	}
	if blog.Visibility != VisibilityPrivate || len(blog.AllowedUserIDs) != 1 {
		t.Errorf("access changed: %+v", blog)
	}
}

// A rejected value leaves the post exactly as it was, rather than half-applied.
func TestDraft_ApplyToInvalid(t *testing.T) {
	blog := Blog{Slug: "hello", OwnerID: "owner", Title: "Hello", Content: "body"}
	draft := Draft{Title: strings.Repeat("a", MaxTitleLength+1), Content: "new body"}

	err := draft.ApplyTo(&blog)

	var invalid ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("ApplyTo = %v, want a ValidationError", err)
	}
	if blog.Title != "Hello" || blog.Content != "body" {
		t.Errorf("blog = %+v, want it untouched", blog)
	}
}
