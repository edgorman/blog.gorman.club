package entity

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeTag(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		want  string
	}{
		{"already normalized", "go", "go"},
		{"case folded", "GoLang", "golang"},
		{"spaces become one hyphen", "Web  Dev", "web-dev"},
		{"hyphens are the same tag as spaces", "WEB-DEV", "web-dev"},
		{"punctuation is a word boundary", "c++/rust!", "c-rust"},
		{"surrounding whitespace goes", "  go\n", "go"},
		{"digits are kept", "go2", "go2"},
		// Unlike a slug, which has to survive being read off a screen, a tag is only ever
		// followed from a link the post itself rendered - so nothing is transliterated away.
		{"non-ASCII letters are kept", "Café", "café"},
		{"other scripts are kept", "日本語", "日本語"},
		{"nothing to normalize", "!!!", ""},
		{"empty", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeTag(tt.input); got != tt.want {
				t.Errorf("NormalizeTag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBlog_SetTags(t *testing.T) {
	var blog Blog

	if err := blog.SetTags([]string{"Go", " web dev ", "go", "!!!", "GO"}); err != nil {
		t.Fatalf("SetTags = %v, want no error", err)
	}
	// Normalized on the way in, and de-duplicated after normalizing rather than before - "Go",
	// "go" and "GO" are one tag, and the one that names nothing is dropped entirely.
	want := []string{"go", "web-dev"}
	if !slices.Equal(blog.Tags, want) {
		t.Errorf("Tags = %v, want %v", blog.Tags, want)
	}

	if err := blog.SetTags(nil); err != nil {
		t.Fatalf("SetTags(nil) = %v, want no error", err)
	}
	if blog.Tags != nil {
		t.Errorf("Tags = %v, want nil once cleared", blog.Tags)
	}
}

func TestBlog_SetTagsRejectsTooManyAndTooLong(t *testing.T) {
	tooMany := make([]string, MaxTags+1)
	for i := range tooMany {
		tooMany[i] = string(rune('a' + i))
	}

	for _, tt := range []struct {
		name  string
		input []string
	}{
		{"too many", tooMany},
		{"one too long", []string{strings.Repeat("a", MaxTagLength+1)}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			blog := Blog{Tags: []string{"kept"}}
			err := blog.SetTags(tt.input)

			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("SetTags = %v, want a ValidationError", err)
			}
			if invalid.Field != "tags" {
				t.Errorf("Field = %q, want %q", invalid.Field, "tags")
			}
			if !slices.Equal(blog.Tags, []string{"kept"}) {
				t.Errorf("Tags = %v, want the rejected value not to be applied", blog.Tags)
			}
		})
	}
}

// A tag is counted in characters rather than bytes, like every other length rule here, so a tag
// written in a multi-byte script is not refused for being shorter than it looks.
func TestBlog_SetTagsCountsCharacters(t *testing.T) {
	var blog Blog
	tag := strings.Repeat("日", MaxTagLength)

	if err := blog.SetTags([]string{tag}); err != nil {
		t.Fatalf("SetTags = %v, want no error", err)
	}
	if !slices.Equal(blog.Tags, []string{tag}) {
		t.Errorf("Tags = %v, want %v", blog.Tags, []string{tag})
	}
}

func TestBlog_HasTag(t *testing.T) {
	blog := Blog{Tags: []string{"go", "web-dev"}}

	for _, tt := range []struct {
		input string
		want  bool
	}{
		{"go", true},
		// A tag arrives from a query string, where nothing has normalized it yet.
		{"Go", true},
		{"Web Dev", true},
		{"web-dev", true},
		{"rust", false},
		{"", false},
		{"!!!", false},
	} {
		t.Run(tt.input, func(t *testing.T) {
			if got := blog.HasTag(tt.input); got != tt.want {
				t.Errorf("HasTag(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}

	var untagged Blog
	if untagged.HasTag("go") {
		t.Error("HasTag on an untagged post = true, want false")
	}
}

func TestBlog_MatchesQuery(t *testing.T) {
	blog := Blog{Title: "Hello, World", Content: "A post about Firestore.", Tags: []string{"go"}}

	for _, tt := range []struct {
		name  string
		input string
		want  bool
	}{
		{"empty matches everything", "", true},
		{"whitespace only matches everything", "   ", true},
		{"title substring", "hello", true},
		{"title ignores case", "WORLD", true},
		{"content substring", "firestore", true},
		{"query is trimmed", "  firestore  ", true},
		{"mid-word is still a match", "irest", true},
		{"absent", "rust", false},
		// Tags have an exact filter of their own, so a post about Go does not surface for
		// every query that happens to be a prefix of one of its topics.
		{"tags are not searched", "go", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := blog.MatchesQuery(tt.input); got != tt.want {
				t.Errorf("MatchesQuery(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
