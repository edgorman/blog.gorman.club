package entity

import (
	"errors"
	"strings"
	"testing"
)

// The point of the rule is that any emoji works, not a fixed five - so this asserts the shape of
// what is admitted rather than a list of blessed glyphs, and pins the things that must not be.
func TestValidEmoji(t *testing.T) {
	for _, tt := range []struct {
		name  string
		emoji string
		want  bool
	}{
		{"a plain pictograph", "👍", true},
		{"one with a variation selector", "❤️", true},
		{"one outside the emoji blocks proper", "⭐", true},
		{"a dingbat", "✅", true},
		{"a skin tone modifier", "👍🏽", true},
		{"a joined sequence", "👨‍👩‍👧‍👦", true},
		{"a flag", "🇬🇧", true},
		{"something recent enough to postdate this rule", "🫠", true},
		{"empty", "", false},
		{"a word", "nice", false},
		{"a word with an emoji in it", "nice 👍", false},
		{"a digit", "1", false},
		{"only a modifier", "‍", false},
		{"two emoji", "👍👎", false},
		{"two emoji joined by nothing but a variation selector", "👍️👎", false},
		{"two flags", "🇬🇧🇺🇸", false},
		{"a skin tone with nothing to recolour", "🏽", false},
		{"a run of them", strings.Repeat("👍", MaxEmojiRunes+1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidEmoji(tt.emoji); got != tt.want {
				t.Errorf("ValidEmoji(%q) = %v, want %v", tt.emoji, got, tt.want)
			}
		})
	}
}

// A key is what makes a reaction unique, so the thing to prove is that two different reactions can
// never produce one - across both kinds of target, and between them.
func TestReaction_KeysAreDistinct(t *testing.T) {
	keys := make(map[string]string)
	for _, tt := range []struct {
		name     string
		reaction Reaction
	}{
		{"the post, one reader", Reaction{Target: PostReaction("hello-world"), UID: "reader"}},
		{"the post, another reader", Reaction{Target: PostReaction("hello-world"), UID: "other"}},
		{"a comment, one reader", Reaction{Target: CommentReaction("hello-world", "cmt1"), UID: "reader"}},
		{"another comment, one reader", Reaction{Target: CommentReaction("hello-world", "cmt2"), UID: "reader"}},
		{"a comment, another reader", Reaction{Target: CommentReaction("hello-world", "cmt1"), UID: "other"}},
		// A comment id is letters and digits alone, so a uid that looks like part of one still
		// cannot be read as a different comment.
		{"a reader whose uid looks like a comment", Reaction{Target: PostReaction("hello-world"), UID: "cmt1-reader"}},
	} {
		key := tt.reaction.Key()
		if previous, clash := keys[key]; clash {
			t.Errorf("%s and %s both key to %q", tt.name, previous, key)
		}
		keys[key] = tt.name
	}
}

func TestReactionTarget_Validate(t *testing.T) {
	for _, tt := range []struct {
		name   string
		target ReactionTarget
		valid  bool
	}{
		{"a post", PostReaction("hello-world"), true},
		{"a comment", CommentReaction("hello-world", "cmt1"), true},
		{"no post", PostReaction(""), false},
		{"a malformed slug", PostReaction("hello world"), false},
		{"a malformed comment id", CommentReaction("hello-world", "cmt/1"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.Validate()

			if tt.valid {
				if err != nil {
					t.Fatalf("Validate = %v, want no error", err)
				}
				return
			}
			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("Validate = %v, want a ValidationError", err)
			}
		})
	}
}

func TestReaction_Validate(t *testing.T) {
	for _, tt := range []struct {
		name     string
		reaction Reaction
		valid    bool
	}{
		{"one emoji", Reaction{Target: PostReaction("hello-world"), UID: "reader", Emojis: []string{"👍"}}, true},
		{"none yet", Reaction{Target: PostReaction("hello-world"), UID: "reader"}, true},
		{"no reader", Reaction{Target: PostReaction("hello-world"), Emojis: []string{"👍"}}, false},
		{"a blank reader", Reaction{Target: PostReaction("hello-world"), UID: "  ", Emojis: []string{"👍"}}, false},
		{"not an emoji", Reaction{Target: PostReaction("hello-world"), UID: "reader", Emojis: []string{"nice"}}, false},
		{"the same emoji twice", Reaction{Target: PostReaction("hello-world"), UID: "reader", Emojis: []string{"👍", "👍"}}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.reaction.Validate()

			if tt.valid {
				if err != nil {
					t.Fatalf("Validate = %v, want no error", err)
				}
				return
			}
			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("Validate = %v, want a ValidationError", err)
			}
			if invalid.Field == "" {
				t.Error("ValidationError does not name a field")
			}
		})
	}
}

func TestReaction_AddAndRemove(t *testing.T) {
	reaction := Reaction{Target: PostReaction("hello-world"), UID: "reader"}

	changed, err := reaction.Add("👍")
	if err != nil || !changed {
		t.Fatalf("Add = %v, %v, want it to have been added", changed, err)
	}

	// The same button sends the same request whatever the stored state, so a second click from a
	// stale page is answered rather than refused.
	changed, err = reaction.Add("👍")
	if err != nil || changed {
		t.Errorf("Add of one already there = %v, %v, want no change and no error", changed, err)
	}
	if len(reaction.Emojis) != 1 {
		t.Errorf("Emojis = %v, want the duplicate to have been dropped", reaction.Emojis)
	}

	if _, err := reaction.Add("nice"); err == nil {
		t.Error("Add of a non-emoji = nil, want a ValidationError")
	}

	if !reaction.Remove("👍") {
		t.Error("Remove = false, want the emoji to have been taken back")
	}
	if !reaction.IsEmpty() {
		t.Errorf("Emojis = %v, want none left", reaction.Emojis)
	}
	if reaction.Remove("👍") {
		t.Error("Remove of one that was never there = true, want no change")
	}
}

// The bound is per reader, so one enthusiast cannot fill the bar by themselves - while any number
// of readers may still each add their own.
func TestReaction_AddIsBoundedPerReader(t *testing.T) {
	reaction := Reaction{Target: PostReaction("hello-world"), UID: "reader"}
	emojis := []string{"👍", "👎", "😀", "😂", "😍", "🎉", "🔥", "💯", "🚀", "👀", "🙏", "✅", "⭐"}

	for _, emoji := range emojis[:MaxReactionsPerTarget] {
		if _, err := reaction.Add(emoji); err != nil {
			t.Fatalf("Add(%q) = %v, want no error below the bound", emoji, err)
		}
	}

	if _, err := reaction.Add(emojis[MaxReactionsPerTarget]); err == nil {
		t.Error("Add past the bound = nil, want a ValidationError")
	}
	if len(reaction.Emojis) != MaxReactionsPerTarget {
		t.Errorf("Emojis = %d, want the refused one not to have been added", len(reaction.Emojis))
	}
}
