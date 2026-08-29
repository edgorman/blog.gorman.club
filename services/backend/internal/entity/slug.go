package entity

import (
	"math/rand/v2"
	"strings"
)

const (
	// blogSlugSeparator joins the words of a slug, and the slug to the suffix below. It is not
	// itself a letter or digit, so blogSlugPattern has to admit it too.
	blogSlugSeparator = "-"
	// blogSlugSuffixLength is how many characters are appended when the plain slug for a title is
	// already taken. Five from the alphabet below is about 28 million slugs per title, which is
	// far more than one title will ever need across every author.
	blogSlugSuffixLength = 5
	// maxBlogSlugBaseLength caps the title-derived part of a slug. Titles run to MaxTitleLength
	// (200) characters, which makes an unabridged slug a poor URL, so a long one is cut at the
	// last whole word that fits.
	maxBlogSlugBaseLength = 80
	// MaxBlogSlugLength is the widest slug NewUniqueBlogSlug can produce: a full-length base, the
	// separator, and the suffix.
	MaxBlogSlugLength = maxBlogSlugBaseLength + len(blogSlugSeparator) + blogSlugSuffixLength
	// untitledBlogSlug stands in for a title no slug survives - one that is empty, or written
	// entirely in a script the ASCII-only alphabet cannot carry. Posting does not require a title
	// (the frontend renders those as "(untitled)"), so every such post lands here and is told
	// apart by the suffix, exactly as two posts sharing a real title would be.
	untitledBlogSlug = "untitled"
)

// blogSlugAlphabet is what a suffix is drawn from: lowercase letters and digits, less the pairs
// that are easily confused when a URL is read off a screen or dictated ("i"/"l"/"1", "o"/"0").
const blogSlugAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// reservedBlogSlugs are slugs no post may hold, because a route already means something else at
// that path: "new" is the frontend's editor at /post/new, which outranks the slug wildcard beside
// it, so a post there would be unreachable at its own URL.
//
// A reserved slug behaves like one permanently taken: NewBlogSlug hands back the suffixed form
// instead of it, and SetSlug refuses it outright so nothing assembled elsewhere can hold one.
var reservedBlogSlugs = map[string]bool{"new": true}

// blogSlugFrom reduces a title to the URL-safe form of itself: lowercase, ASCII, one hyphen
// between words. Anything else - punctuation, spaces, accents, other scripts - is a word boundary
// rather than something to transliterate, since a slug is a URL first and a rendering of the title
// second.
//
// The title itself is stored untouched, so nothing here is lossy for the reader; a post whose
// title slugs to nothing gets untitledBlogSlug instead of an empty (and unaddressable) slug.
func blogSlugFrom(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})

	var slug strings.Builder
	for _, word := range words {
		// A single word longer than the cap is cut mid-word rather than dropped, which is what
		// keeps a title of one very long run of characters from slugging to nothing. Every word
		// here is ASCII by construction, so cutting by byte cannot split a character.
		if slug.Len() == 0 {
			slug.WriteString(word[:min(len(word), maxBlogSlugBaseLength)])
			continue
		}
		if slug.Len()+len(blogSlugSeparator)+len(word) > maxBlogSlugBaseLength {
			break
		}
		slug.WriteString(blogSlugSeparator)
		slug.WriteString(word)
	}

	if slug.Len() == 0 {
		return untitledBlogSlug
	}
	return slug.String()
}

// NewBlogSlug returns the slug a post takes when nothing else holds it: the title, slugified. It
// is what every post is tried under first, so the common case - the first post anywhere under a
// title - reads as itself in the URL.
//
// Slugs are unique across every author (see the repository's document key), which is what lets a
// post be addressed as /post/{slug} with no author beside it. So the second post under a title
// falls back to the suffixed form below whoever wrote it, rather than only the same author's.
func NewBlogSlug(title string) string {
	slug := blogSlugFrom(title)
	// A reserved slug is one nothing can ever hold, so a post titled into it goes straight to the
	// suffixed form rather than spending an attempt on a base that is refused either way.
	if reservedBlogSlugs[slug] {
		return NewUniqueBlogSlug(title)
	}
	return slug
}

// NewUniqueBlogSlug returns the same slug with a random suffix, e.g. "hello-world-k3m9x". It is
// what a post falls back to when some post already holds NewBlogSlug's plain form.
//
// The suffix always carries a separator, so the result can never be one of reservedBlogSlugs -
// every name there is a single word.
//
// The suffix is drawn rather than counted up ("-2", "-3") deliberately: counting would need a read
// of every other post under the title to know where to start, and would still race with a
// concurrent post.
// A draw needs neither, since only the write itself can decide whether a slug is free - so a
// caller draws again on repository.ErrSlugTaken instead of trusting a single attempt.
func NewUniqueBlogSlug(title string) string {
	suffix := make([]byte, blogSlugSuffixLength)
	for i := range suffix {
		suffix[i] = blogSlugAlphabet[rand.IntN(len(blogSlugAlphabet))]
	}
	return blogSlugFrom(title) + blogSlugSeparator + string(suffix)
}
