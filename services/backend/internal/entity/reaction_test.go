package entity

import (
	"errors"
	"testing"
)

// Reactions are a fixed set of five, not any emoji: ValidEmoji is exact membership in
// AllowedEmojis, so nothing close to one of them - a composed variant, a lookalike, a bare word -
// is accepted just because it resembles a member of the set.
func TestValidEmoji(t *testing.T) {
	for _, allowed := range AllowedEmojis {
		t.Run(allowed, func(t *testing.T) {
			if !ValidEmoji(allowed) {
				t.Errorf("ValidEmoji(%q) = false, want true - it is in AllowedEmojis", allowed)
			}
		})
	}

	for _, tt := range []struct {
		name  string
		emoji string
	}{
		{"empty", ""},
		{"a word", "nice"},
		{"a word with an emoji in it", "nice 👍"},
		{"an emoji not in the set", "🎊"},
		{"a skin-toned variant of an allowed emoji", "👍🏽"},
		{"two allowed emoji together", "👍👎"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if ValidEmoji(tt.emoji) {
				t.Errorf("ValidEmoji(%q) = true, want false", tt.emoji)
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

// A reader is naturally bounded by the size of AllowedEmojis: once every one of the five is
// chosen, there is nothing left to Add that Validate would accept - no separate limit is needed.
func TestReaction_AddAllFiveThenNoMore(t *testing.T) {
	reaction := Reaction{Target: PostReaction("hello-world"), UID: "reader"}

	for _, emoji := range AllowedEmojis {
		if _, err := reaction.Add(emoji); err != nil {
			t.Fatalf("Add(%q) = %v, want no error", emoji, err)
		}
	}
	if len(reaction.Emojis) != len(AllowedEmojis) {
		t.Errorf("Emojis = %d, want all %d allowed", len(reaction.Emojis), len(AllowedEmojis))
	}

	if _, err := reaction.Add("🎊"); err == nil {
		t.Error("Add of an emoji outside the set = nil, want a ValidationError")
	}
}
