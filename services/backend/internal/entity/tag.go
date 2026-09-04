package entity

import (
	"strings"
	"unicode"
)

const (
	// MaxTags caps how many topics one post may carry. A tag is a way of finding a post among
	// others, so a post under twenty of them is tagged with nothing in particular.
	MaxTags = 10
	// MaxTagLength caps one tag, counted in characters. It is far shorter than a title because a
	// tag is a label a reader scans a row of, not a sentence.
	MaxTagLength = 32
	// tagSeparator joins the words of a normalized tag, exactly as blogSlugSeparator joins the
	// words of a slug.
	tagSeparator = "-"
)

// NormalizeTag reduces what an author typed to the single form a tag is stored, filtered, and
// linked under: lowercase, one hyphen between words, and nothing else. A tag is half a label and
// half an address - it is what `?tag=` names - so "Web Dev", "web dev" and "WEB-DEV" have to be
// one tag rather than three that merely look alike.
//
// Unlike a slug this keeps every script rather than only ASCII: a slug has to survive being read
// off a screen and typed back in, while a tag is only ever followed from a link the post itself
// rendered, so there is nothing to gain by discarding an author's own language.
//
// It returns "" for anything that reduces to nothing - punctuation alone, or whitespace - which
// is what SetTags drops rather than stores, since such a tag names no topic at all.
func NormalizeTag(tag string) string {
	words := strings.FieldsFunc(strings.ToLower(tag), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.Join(words, tagSeparator)
}
