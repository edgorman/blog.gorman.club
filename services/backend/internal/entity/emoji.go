package entity

import genblogv1 "github.com/edgorman/blog.gorman.club/services/backend/internal/gen/blog/v1"

// AllowedEmojis is the whole set a reaction may be. It is a fixed list rather than a rule about
// shape: five choices are what a reaction bar needs to stay a row of chips rather than a keyboard,
// and a fixed set is what keeps every reader's bar built from the same five glyphs instead of an
// open-ended scatter of one-off combinations. Widening or restyling the set is a change to this
// array and nothing else - nothing downstream assumes what is in it, only that ValidEmoji agrees.
//
// It is read off the generated blog.v1.AllowedEmojis message (packages/protos/blog/v1/emoji.proto)
// rather than written as a plain literal, so the shape a client eventually decodes off the wire is
// the same shape this package validates against - see `packages/protos/CLAUDE.md`'s "Contract Layer" section.
var AllowedEmojis = (&genblogv1.AllowedEmojis{
	Emoji: []string{"👍", "👎", "❤️", "😄", "🎉"},
}).GetEmoji()

// ValidEmoji reports whether s is one of AllowedEmojis - exactly, not a prefix or a composed
// variant of one. There is no custom-emoji upload and no combining runes of your own: a skin tone,
// a flag, or a joined family is a different string than the plain glyph it modifies, so none of
// them match.
func ValidEmoji(s string) bool {
	for _, allowed := range AllowedEmojis {
		if s == allowed {
			return true
		}
	}
	return false
}
