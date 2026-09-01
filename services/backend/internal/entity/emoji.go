package entity

import "unicode/utf8"

// MaxEmojiRunes bounds one emoji. A single pictograph is one or two runes, but a composed sequence
// is legitimately longer - a family (👨‍👩‍👧‍👦) is seven, and a skin-toned handshake with a
// variation selector reaches eight - so the bound is what the longest real sequence needs and not
// one rune more. It is what stops a caller sending a paragraph of pictographs as "an emoji".
const MaxEmojiRunes = 8

// emojiRanges are the blocks a reaction may be drawn from. This is deliberately a range check
// rather than a list of permitted emoji: the point of it is that any emoji works, so the rule has
// to admit ones that did not exist when it was written. It is checked at all only because the
// alternative - accepting any string - would let a reaction be a word, and the whole shape of the
// feature (a chip beside a count) assumes a glyph.
//
// The ranges are broader than the emoji they contain: a few unassigned or non-emoji symbols slip
// through, which costs nothing. What must not slip through is ordinary text, and none of these
// blocks holds any.
var emojiRanges = [][2]rune{
	{0x00A9, 0x00A9},   // ©
	{0x00AE, 0x00AE},   // ®
	{0x203C, 0x2049},   // ‼ ⁉
	{0x2122, 0x2122},   // ™
	{0x2194, 0x21AA},   // arrows
	{0x231A, 0x231B},   // ⌚ ⌛
	{0x2328, 0x2328},   // ⌨
	{0x23CF, 0x23FA},   // media controls, ⏰
	{0x24C2, 0x24C2},   // Ⓜ
	{0x25AA, 0x25FE},   // small geometric shapes
	{0x2600, 0x27BF},   // misc symbols and dingbats: ☀ ✅ ➡
	{0x2934, 0x2935},   // ⤴ ⤵
	{0x2B00, 0x2BFF},   // misc symbols and arrows: ⬆ ⭐
	{0x3030, 0x3030},   // 〰
	{0x303D, 0x303D},   // 〽
	{0x3297, 0x3299},   // ㊗ ㊙
	{0x1F000, 0x1FAFF}, // the emoji blocks proper, flags and skin tones included
}

// emojiModifiers are the runes that only ever qualify an emoji beside them: the zero-width joiner
// that builds a sequence, the variation selectors that ask for the emoji or text presentation, and
// the keycap combiner. They are admitted anywhere in an emoji but never count as one on their own,
// which is what stops an "emoji" made entirely of invisible characters.
func isEmojiModifier(r rune) bool {
	switch r {
	case 0x200D, 0xFE0E, 0xFE0F, 0x20E3:
		return true
	}
	return false
}

// isSkinTone reports whether r is one of the five tone modifiers, which follow the pictograph they
// recolour rather than standing beside it.
func isSkinTone(r rune) bool {
	return r >= 0x1F3FB && r <= 0x1F3FF
}

// isRegionalIndicator reports whether r is one of the 26 letters flags are spelled with. They are
// the one case where two pictographs are genuinely one emoji without a joiner between them, so
// they are counted rather than merely admitted: two make a flag, four make two flags.
func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

func isEmojiRune(r rune) bool {
	for _, span := range emojiRanges {
		if r >= span[0] && r <= span[1] {
			return true
		}
	}
	return false
}

// ValidEmoji reports whether s is a *single* emoji this service will store: every rune is either
// an emoji or something that qualifies one, at least one is an emoji in its own right, and the
// whole reads as one glyph rather than several set side by side.
//
// That last part is the reason this is not simply a rune count. "👍👎" is two runes of perfectly
// good emoji, and admitting it would let a reader invent chips nobody else can reproduce and
// fragment a post's reaction bar into one-off combinations. So a second pictograph is admitted
// only where it really is part of one glyph: after a zero-width joiner (👨‍👩‍👧‍👦), as a skin
// tone (👍🏽), or as the other half of a flag (🇬🇧).
//
// Keycaps (#️⃣, 1️⃣) are the one common emoji this refuses, because admitting them would mean
// admitting the ASCII digits and "#" they are built from - and with them, "1" as a reaction.
func ValidEmoji(s string) bool {
	if s == "" || utf8.RuneCountInString(s) > MaxEmojiRunes {
		return false
	}

	base := false
	regionals := 0
	var previous rune
	for i, r := range s {
		switch {
		case isEmojiModifier(r):
		case isRegionalIndicator(r):
			// Two spell one flag; a third starts a second flag, which is two emoji.
			if regionals++; regionals > 2 {
				return false
			}
			base = true
		case isSkinTone(r):
			// A tone recolours the pictograph before it, so on its own it is not an emoji.
			if !base {
				return false
			}
		case isEmojiRune(r):
			// Every pictograph after the first has to be joined to what came before it, or the
			// two are simply two emoji.
			if i > 0 && previous != 0x200D {
				return false
			}
			base = true
		default:
			return false
		}
		previous = r
	}
	return base
}
