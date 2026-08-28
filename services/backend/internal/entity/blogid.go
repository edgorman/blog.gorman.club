package entity

import (
	"math/rand/v2"
	"strings"
)

const (
	// blogIDSeparator joins the words of a slug, and the slug to the suffix below. It is not
	// itself a letter or digit, so blogIDPattern has to admit it too.
	blogIDSeparator = "-"
	// blogIDSuffixLength is how many characters are appended when a title's plain slug is already
	// held. Five from the alphabet below is about 28 million ids per title - enough that a post
	// competing for a busy title still finds a free one in a couple of draws.
	blogIDSuffixLength = 5
	// maxBlogSlugLength caps the title-derived part of an id. Titles run to MaxTitleLength (200)
	// characters, which makes an unabridged slug a poor URL, so a long one is cut at the last
	// whole word that fits.
	maxBlogSlugLength = 80
	// MaxBlogIDLength is the widest id NewUniqueBlogID can produce: a full-length slug, the
	// separator, and the suffix.
	MaxBlogIDLength = maxBlogSlugLength + len(blogIDSeparator) + blogIDSuffixLength
	// untitledBlogSlug stands in for a title no slug survives - one that is empty, or written
	// entirely in a script the ASCII-only alphabet cannot carry. Posting does not require a title
	// (the frontend renders those as "(untitled)"), so every such post lands here and is told
	// apart by the suffix, exactly as two posts sharing a real title would be.
	untitledBlogSlug = "untitled"
)

// blogIDAlphabet is what a suffix is drawn from: lowercase letters and digits, less the pairs that
// are easily confused when an id is read off a screen or dictated ("i"/"l"/"1", "o"/"0").
const blogIDAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// blogSlug reduces a title to the id-safe form of itself: lowercase, ASCII, one hyphen between
// words. Anything else - punctuation, spaces, accents, other scripts - is a word boundary rather
// than something to transliterate, since an id is a URL first and a rendering of the title second.
//
// The title itself is stored untouched, so nothing here is lossy for the reader; a post whose
// title slugs to nothing gets untitledBlogSlug instead of an empty (and unaddressable) id.
func blogSlug(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})

	var slug strings.Builder
	for _, word := range words {
		// A single word longer than the cap is cut mid-word rather than dropped, which is what
		// keeps a title of one very long run of characters from slugging to nothing. Every word
		// here is ASCII by construction, so cutting by byte cannot split a character.
		if slug.Len() == 0 {
			slug.WriteString(word[:min(len(word), maxBlogSlugLength)])
			continue
		}
		if slug.Len()+len(blogIDSeparator)+len(word) > maxBlogSlugLength {
			break
		}
		slug.WriteString(blogIDSeparator)
		slug.WriteString(word)
	}

	if slug.Len() == 0 {
		return untitledBlogSlug
	}
	return slug.String()
}

// NewBlogID returns the id a post takes when nothing else holds it: the title, slugified. It is
// what every post is tried under first, so the common case - a title nobody has used - reads as
// itself in the URL.
func NewBlogID(title string) string {
	return blogSlug(title)
}

// NewUniqueBlogID returns the same slug with a random suffix, e.g. "hello-world-k3m9x". It is what
// a post falls back to when NewBlogID's plain form is already taken.
//
// The suffix is drawn rather than counted up ("-2", "-3") deliberately: counting would need a read
// of every neighbouring id to know where to start, and would still race with a concurrent post. A
// draw needs neither, since only the write itself can decide whether an id is free - so a caller
// draws again on repository.ErrBlogIDTaken instead of trusting a single attempt.
func NewUniqueBlogID(title string) string {
	suffix := make([]byte, blogIDSuffixLength)
	for i := range suffix {
		suffix[i] = blogIDAlphabet[rand.IntN(len(blogIDAlphabet))]
	}
	return blogSlug(title) + blogIDSeparator + string(suffix)
}
