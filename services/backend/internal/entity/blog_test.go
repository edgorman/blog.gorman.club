package entity

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBlog_SetVisibility(t *testing.T) {
	for _, tt := range []struct {
		input Visibility
		valid bool
	}{
		{VisibilityPublic, true},
		{VisibilityPrivate, true},
		{"everyone", false},
		{"", false},
		{"Public", false},
	} {
		t.Run(string(tt.input), func(t *testing.T) {
			blog := Blog{Visibility: VisibilityPublic}
			err := blog.SetVisibility(tt.input)

			if tt.valid {
				if err != nil {
					t.Fatalf("SetVisibility(%q) = %v, want no error", tt.input, err)
				}
				if blog.Visibility != tt.input {
					t.Errorf("Visibility = %q, want %q", blog.Visibility, tt.input)
				}
				return
			}

			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("SetVisibility(%q) = %v, want a ValidationError", tt.input, err)
			}
			if blog.Visibility != VisibilityPublic {
				t.Errorf("Visibility = %q, want the rejected value not to be applied", blog.Visibility)
			}
		})
	}
}

// An empty title is legitimate - the frontend renders those as "(untitled)".
func TestBlog_SetTitle(t *testing.T) {
	var blog Blog

	if err := blog.SetTitle("  Hello  "); err != nil {
		t.Fatalf("SetTitle = %v, want no error", err)
	}
	if blog.Title != "Hello" {
		t.Errorf("Title = %q, want %q", blog.Title, "Hello")
	}

	if err := blog.SetTitle(""); err != nil {
		t.Errorf("SetTitle(\"\") = %v, want no error", err)
	}

	if err := blog.SetTitle(strings.Repeat("a", MaxTitleLength+1)); err == nil {
		t.Error("SetTitle over the limit = nil, want a ValidationError")
	}
}

// Whitespace is significant in post bodies, so content is not trimmed.
func TestBlog_SetContent_PreservesWhitespace(t *testing.T) {
	var blog Blog

	if err := blog.SetContent("\n  indented\n"); err != nil {
		t.Fatalf("SetContent = %v, want no error", err)
	}
	if blog.Content != "\n  indented\n" {
		t.Errorf("Content = %q, want the whitespace preserved", blog.Content)
	}

	if err := blog.SetContent(strings.Repeat("a", MaxContentLength+1)); err == nil {
		t.Error("SetContent over the limit = nil, want a ValidationError")
	}
}

func TestBlog_SetAllowedUserIDs(t *testing.T) {
	var blog Blog

	if err := blog.SetAllowedUserIDs([]string{"a", "  b  ", "", "   ", "a"}); err != nil {
		t.Fatalf("SetAllowedUserIDs = %v, want no error", err)
	}
	if want := []string{"a", "b"}; !slices.Equal(blog.AllowedUserIDs, want) {
		t.Errorf("AllowedUserIDs = %v, want %v (blanks and duplicates dropped)", blog.AllowedUserIDs, want)
	}

	// Nothing left after cleaning means the field is absent, not an empty array.
	if err := blog.SetAllowedUserIDs([]string{"", "  "}); err != nil {
		t.Fatalf("SetAllowedUserIDs = %v, want no error", err)
	}
	if blog.AllowedUserIDs != nil {
		t.Errorf("AllowedUserIDs = %v, want nil", blog.AllowedUserIDs)
	}

	tooMany := make([]string, MaxAllowedUsers+1)
	for i := range tooMany {
		tooMany[i] = string(rune('a'+i%26)) + strings.Repeat("x", i)
	}
	if err := blog.SetAllowedUserIDs(tooMany); err == nil {
		t.Error("SetAllowedUserIDs over the limit = nil, want a ValidationError")
	}
}

// De-duplication is quadratic, so the size check has to come first: cleaning an unbounded list
// before rejecting it lets a caller burn arbitrary CPU on a request that fails anyway. A huge
// input must be refused in constant time rather than after the scan.
func TestSetAllowedUserIDs_RejectsOversizedInputWithoutScanningIt(t *testing.T) {
	huge := make([]string, 200_000)
	for i := range huge {
		huge[i] = fmt.Sprintf("uid-%d", i)
	}

	var blog Blog
	done := make(chan error, 1)
	go func() { done <- blog.SetAllowedUserIDs(huge) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("SetAllowedUserIDs = nil for an oversized list, want a ValidationError")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SetAllowedUserIDs did not reject an oversized list promptly - it is scanning the input before checking its size")
	}
}

func TestBlog_CanBeReadBy(t *testing.T) {
	for _, tt := range []struct {
		name string
		blog Blog
		uid  string
		want bool
	}{
		{"public post, any caller", Blog{OwnerID: "owner", Visibility: VisibilityPublic}, "someone", true},
		{"private post, owner", Blog{OwnerID: "owner", Visibility: VisibilityPrivate}, "owner", true},
		{"private post, whitelisted", Blog{OwnerID: "owner", Visibility: VisibilityPrivate, AllowedUserIDs: []string{"friend"}}, "friend", true},
		{"private post, stranger", Blog{OwnerID: "owner", Visibility: VisibilityPrivate, AllowedUserIDs: []string{"friend"}}, "stranger", false},
		{"private post, no whitelist", Blog{OwnerID: "owner", Visibility: VisibilityPrivate}, "stranger", false},
		{"unowned private post, empty uid", Blog{Visibility: VisibilityPrivate}, "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.blog.CanBeReadBy(tt.uid); got != tt.want {
				t.Errorf("CanBeReadBy(%q) = %v, want %v", tt.uid, got, tt.want)
			}
		})
	}
}

// An unowned blog must not be claimed by a caller whose uid is also empty.
func TestBlog_IsOwnedBy_EmptyUIDNeverMatches(t *testing.T) {
	if (Blog{}).IsOwnedBy("") {
		t.Error("IsOwnedBy(\"\") = true for an unowned blog, want false")
	}
	if !(Blog{OwnerID: "owner"}).IsOwnedBy("owner") {
		t.Error("IsOwnedBy(owner) = false, want true")
	}
}

func TestBlog_Validate(t *testing.T) {
	valid := Blog{OwnerID: "owner", Title: "Hello", Content: "Body", Visibility: VisibilityPublic}
	if err := valid.Validate(); err != nil {
		t.Errorf("Validate = %v, want no error", err)
	}

	if err := (Blog{OwnerID: "owner", Visibility: "everyone"}).Validate(); err == nil {
		t.Error("Validate with a bad visibility = nil, want an error")
	}
	if err := (Blog{OwnerID: "owner", Title: strings.Repeat("a", MaxTitleLength+1), Visibility: VisibilityPublic}).Validate(); err == nil {
		t.Error("Validate with an overlong title = nil, want an error")
	}

	// An ownerless blog is unwritable by anyone, so it must never reach a repository.
	if err := (Blog{Visibility: VisibilityPublic}).Validate(); err == nil {
		t.Error("Validate with no owner = nil, want an error")
	}
}
