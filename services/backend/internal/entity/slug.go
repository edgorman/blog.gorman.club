package entity

import (
	"math/rand/v2"
	"strings"
)

const (
	// blogSlugSeparator joins the words of a slug, and the slug to the suffix below. It is not
	// itself a letter or digit, so blogSlugPattern has to admit it too.
	blogSlugSeparator = "-"
	// blogSlugSuffixLength is how many characters are appended when an author's own slug for a
	// title is already taken. Five from the alphabet below is about 28 million slugs per title,
	// which is far more than one author will ever need for one title.
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
	// (the frontend renders those as "(untitled)"), so every such post of an author's lands here
	// and is told apart by the suffix, exactly as two posts sharing a real title would be.
	untitledBlogSlug = "untitled"
)

// blogSlugAlphabet is what a suffix is drawn from: lowercase letters and digits, less the pairs
// that are easily confused when a URL is read off a screen or dictated ("i"/"l"/"1", "o"/"0").
const blogSlugAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

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

// NewBlogSlug returns the slug a post takes when its author holds nothing else at it: the title,
// slugified. It is what every post is tried under first, so the common case - an author's first
// post under a title - reads as itself in the URL.
//
// Slugs are unique per author rather than globally (see the repository's document key), so two
// people may both post at "hello-world"; only the same author posting twice under one title has to
// fall back to the suffixed form below.
func NewBlogSlug(title string) string {
	return blogSlugFrom(title)
}

// NewUniqueBlogSlug returns the same slug with a random suffix, e.g. "hello-world-k3m9x". It is
// what a post falls back to when its author already holds NewBlogSlug's plain form.
//
// The suffix is drawn rather than counted up ("-2", "-3") deliberately: counting would need a read
// of the author's other posts to know where to start, and would still race with a concurrent post.
// A draw needs neither, since only the write itself can decide whether a slug is free - so a
// caller draws again on repository.ErrSlugTaken instead of trusting a single attempt.
func NewUniqueBlogSlug(title string) string {
	suffix := make([]byte, blogSlugSuffixLength)
	for i := range suffix {
		suffix[i] = blogSlugAlphabet[rand.IntN(len(blogSlugAlphabet))]
	}
	return blogSlugFrom(title) + blogSlugSeparator + string(suffix)
}
