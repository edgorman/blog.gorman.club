package entity

import (
	"strings"
	"testing"
)

// A generated id goes straight into storage as a document key without passing back through the
// HTTP layer, so no title may produce one SetID would reject.
func assertValidBlogID(t *testing.T, id string) {
	t.Helper()

	var blog Blog
	if err := blog.SetID(id); err != nil {
		t.Fatalf("SetID(%q) = %v, want every generated id to be valid", id, err)
	}
}

func TestNewBlogID_SlugsTheTitle(t *testing.T) {
	for _, tt := range []struct {
		name  string
		title string
		want  string
	}{
		{"plain", "Hello world", "hello-world"},
		{"punctuation", "Hello, world!", "hello-world"},
		{"already a slug", "hello-world", "hello-world"},
		{"runs of separators", "  Hello   ---  world  ", "hello-world"},
		{"digits kept", "Go 1.25 is out", "go-1-25-is-out"},
		{"apostrophes split", "What's new", "what-s-new"},
		{"accents dropped", "Café life", "caf-life"},
		{"empty title", "", untitledBlogSlug},
		{"whitespace only", "   ", untitledBlogSlug},
		{"no ASCII at all", "日本語", untitledBlogSlug},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBlogID(tt.title); got != tt.want {
				t.Errorf("NewBlogID(%q) = %q, want %q", tt.title, got, tt.want)
			}
			assertValidBlogID(t, NewBlogID(tt.title))
		})
	}
}

// A title is capped at MaxTitleLength, which is far longer than an id should be, so a long one is
// cut - at a word boundary where it can be, mid-word only when a single word overruns the cap on
// its own (which is the case that would otherwise leave nothing to cut to).
func TestNewBlogID_TruncatesLongTitles(t *testing.T) {
	t.Run("at a word boundary", func(t *testing.T) {
		id := NewBlogID(strings.TrimSpace(strings.Repeat("word ", 100)))

		if len(id) > maxBlogSlugLength {
			t.Errorf("len(%q) = %d, want at most %d", id, len(id), maxBlogSlugLength)
		}
		if strings.HasSuffix(id, "wor") || strings.HasSuffix(id, "-") {
			t.Errorf("NewBlogID(...) = %q, want it cut after a whole word", id)
		}
		assertValidBlogID(t, id)
	})

	t.Run("mid-word for one long word", func(t *testing.T) {
		id := NewBlogID(strings.Repeat("a", MaxTitleLength))

		if len(id) != maxBlogSlugLength {
			t.Errorf("len(%q) = %d, want %d", id, len(id), maxBlogSlugLength)
		}
		assertValidBlogID(t, id)
	})
}

// The suffixed form is what a post falls back to, so the widest title has to produce an id that is
// still valid and still within MaxBlogIDLength.
func TestNewUniqueBlogID_ShapeAndValidity(t *testing.T) {
	for _, title := range []string{
		"Hello world",
		"",
		"日本語",
		strings.Repeat("a", MaxTitleLength),
		strings.TrimSpace(strings.Repeat("word ", 100)),
	} {
		// Enough draws to exercise a spread of the alphabet while staying a fast unit test.
		for range 100 {
			id := NewUniqueBlogID(title)

			assertValidBlogID(t, id)
			if len(id) > MaxBlogIDLength {
				t.Fatalf("len(%q) = %d, want at most %d", id, len(id), MaxBlogIDLength)
			}

			// The suffixed id has to be the plain one plus a fixed-width suffix, so that a reader
			// seeing either form of a title recognises it as the same post.
			base := NewBlogID(title)
			if want := len(base) + len(blogIDSeparator) + blogIDSuffixLength; len(id) != want {
				t.Fatalf("NewUniqueBlogID(%q) = %q, want %q plus a %d-character suffix", title, id, base, blogIDSuffixLength)
			}
			if !strings.HasPrefix(id, base+blogIDSeparator) {
				t.Fatalf("NewUniqueBlogID(%q) = %q, want it to start with %q", title, id, base)
			}
			if suffix := id[len(id)-blogIDSuffixLength:]; strings.ContainsFunc(suffix, func(r rune) bool {
				return !strings.ContainsRune(blogIDAlphabet, r)
			}) {
				t.Fatalf("suffix %q holds a character outside the alphabet", suffix)
			}
		}
	}
}

// Two draws being equal is possible but should be rare; a generator stuck on one suffix would make
// every post after the second to use a title unplaceable.
func TestNewUniqueBlogID_Varies(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		seen[NewUniqueBlogID("Hello world")] = true
	}

	if len(seen) < 90 {
		t.Errorf("100 draws produced only %d distinct ids, want the alphabet to be doing its job", len(seen))
	}
}

// Ids are read off screens and dictated, so the alphabet deliberately omits the characters that
// are confused when they are - and every character in it has to survive SetID on its own.
func TestBlogIDAlphabetIsWellFormed(t *testing.T) {
	if len(blogIDAlphabet) == 0 {
		t.Fatal("alphabet is empty, which would make every suffix identical")
	}

	seen := make(map[rune]bool, len(blogIDAlphabet))
	for _, r := range blogIDAlphabet {
		if strings.ContainsRune("il1o0", r) {
			t.Errorf("%q is easily confused with another character in the alphabet", r)
		}
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			t.Errorf("%q is not a lowercase letter or digit", r)
		}
		if seen[r] {
			t.Errorf("%q appears twice, which skews the draw towards it", r)
		}
		seen[r] = true
	}
}
