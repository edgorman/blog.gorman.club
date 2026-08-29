package entity

import (
	"strings"
	"testing"
)

// A generated slug goes straight into storage as the whole of a document key without passing back
// through the HTTP layer, so no title may produce one SetSlug would reject.
func assertValidBlogSlug(t *testing.T, slug string) {
	t.Helper()

	var blog Blog
	if err := blog.SetSlug(slug); err != nil {
		t.Fatalf("SetSlug(%q) = %v, want every generated slug to be valid", slug, err)
	}
}

func TestNewBlogSlug_SlugsTheTitle(t *testing.T) {
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
			if got := NewBlogSlug(tt.title); got != tt.want {
				t.Errorf("NewBlogSlug(%q) = %q, want %q", tt.title, got, tt.want)
			}
			assertValidBlogSlug(t, NewBlogSlug(tt.title))
		})
	}
}

// A title is capped at MaxTitleLength, which is far longer than a slug should be, so a long one is
// cut - at a word boundary where it can be, mid-word only when a single word overruns the cap on
// its own (which is the case that would otherwise leave nothing to cut to).
func TestNewBlogSlug_TruncatesLongTitles(t *testing.T) {
	t.Run("at a word boundary", func(t *testing.T) {
		slug := NewBlogSlug(strings.TrimSpace(strings.Repeat("word ", 100)))

		if len(slug) > maxBlogSlugBaseLength {
			t.Errorf("len(%q) = %d, want at most %d", slug, len(slug), maxBlogSlugBaseLength)
		}
		if strings.HasSuffix(slug, "wor") || strings.HasSuffix(slug, "-") {
			t.Errorf("NewBlogSlug(...) = %q, want it cut after a whole word", slug)
		}
		assertValidBlogSlug(t, slug)
	})

	t.Run("mid-word for one long word", func(t *testing.T) {
		slug := NewBlogSlug(strings.Repeat("a", MaxTitleLength))

		if len(slug) != maxBlogSlugBaseLength {
			t.Errorf("len(%q) = %d, want %d", slug, len(slug), maxBlogSlugBaseLength)
		}
		assertValidBlogSlug(t, slug)
	})
}

// The suffixed form is what a post falls back to, so the widest title has to produce a slug that
// is still valid and still within MaxBlogSlugLength.
func TestNewUniqueBlogSlug_ShapeAndValidity(t *testing.T) {
	for _, title := range []string{
		"Hello world",
		"",
		"日本語",
		strings.Repeat("a", MaxTitleLength),
		strings.TrimSpace(strings.Repeat("word ", 100)),
	} {
		// Enough draws to exercise a spread of the alphabet while staying a fast unit test.
		for range 100 {
			slug := NewUniqueBlogSlug(title)

			assertValidBlogSlug(t, slug)
			if len(slug) > MaxBlogSlugLength {
				t.Fatalf("len(%q) = %d, want at most %d", slug, len(slug), MaxBlogSlugLength)
			}

			// The suffixed slug has to be the slugified title plus a fixed-width suffix, so that a
			// reader seeing either form of a title recognises it as the same post. It is compared
			// against blogSlugFrom rather than NewBlogSlug because the two diverge for a title
			// that slugs to a reserved name, which has no plain form.
			base := blogSlugFrom(title)
			if want := len(base) + len(blogSlugSeparator) + blogSlugSuffixLength; len(slug) != want {
				t.Fatalf("NewUniqueBlogSlug(%q) = %q, want %q plus a %d-character suffix", title, slug, base, blogSlugSuffixLength)
			}
			if !strings.HasPrefix(slug, base+blogSlugSeparator) {
				t.Fatalf("NewUniqueBlogSlug(%q) = %q, want it to start with %q", title, slug, base)
			}
			if suffix := slug[len(slug)-blogSlugSuffixLength:]; strings.ContainsFunc(suffix, func(r rune) bool {
				return !strings.ContainsRune(blogSlugAlphabet, r)
			}) {
				t.Fatalf("suffix %q holds a character outside the alphabet", suffix)
			}
		}
	}
}

// Two draws being equal is possible but should be rare; a generator stuck on one suffix would make
// every post after the second under a title unplaceable.
func TestNewUniqueBlogSlug_Varies(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		seen[NewUniqueBlogSlug("Hello world")] = true
	}

	if len(seen) < 90 {
		t.Errorf("100 draws produced only %d distinct ids, want the alphabet to be doing its job", len(seen))
	}
}

// Slugs are read off screens and dictated, so the alphabet deliberately omits the characters that
// are confused when they are - and every character in it has to survive SetSlug on its own.
func TestBlogSlugAlphabetIsWellFormed(t *testing.T) {
	if len(blogSlugAlphabet) == 0 {
		t.Fatal("alphabet is empty, which would make every suffix identical")
	}

	seen := make(map[rune]bool, len(blogSlugAlphabet))
	for _, r := range blogSlugAlphabet {
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

// A slug the frontend claims for a route of its own is one no post can ever hold, so a title that
// slugs to it takes the suffixed form straight away rather than being assigned a name it would
// then be refused - and be unreachable at, since the route wins over the slug wildcard beside it.
func TestNewBlogSlug_AvoidsReservedSlugs(t *testing.T) {
	if len(reservedBlogSlugs) == 0 {
		t.Fatal("no slugs are reserved, which would make this test vacuous")
	}

	for reserved := range reservedBlogSlugs {
		var blog Blog
		if err := blog.SetSlug(reserved); err == nil {
			t.Errorf("SetSlug(%q) = nil, want a reserved slug refused", reserved)
		}

		// The title is the reserved word itself, which is the only way to slug into one.
		slug := NewBlogSlug(reserved)
		if slug == reserved {
			t.Errorf("NewBlogSlug(%q) = %q, want a slug the route does not already claim", reserved, slug)
		}
		if !strings.HasPrefix(slug, reserved+blogSlugSeparator) {
			t.Errorf("NewBlogSlug(%q) = %q, want the title kept and a suffix added", reserved, slug)
		}
		assertValidBlogSlug(t, slug)
	}
}
