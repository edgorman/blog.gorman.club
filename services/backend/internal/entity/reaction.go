package entity

import (
	"slices"
	"strings"
	"time"
)

// ReactionTarget names the thing being reacted to. A reaction is always attached to something
// under one post - the post itself, or one comment on it - so the post's slug is half of every
// target, and CommentID is what tells the two apart: empty means the post.
//
// Both live under the post because that is how they are read: a reader opening a post wants every
// reaction on the page, and one query answers that only if the comments' reactions sit beneath the
// same post as the post's own.
type ReactionTarget struct {
	BlogSlug  string `json:"blogSlug"`
	CommentID string `json:"commentId,omitempty"`
}

// PostReaction returns the target naming a post itself.
func PostReaction(blogSlug string) ReactionTarget {
	return ReactionTarget{BlogSlug: blogSlug}
}

// CommentReaction returns the target naming one comment on a post.
func CommentReaction(blogSlug, commentID string) ReactionTarget {
	return ReactionTarget{BlogSlug: blogSlug, CommentID: commentID}
}

// IsComment reports whether the target is a comment rather than the post itself.
func (t ReactionTarget) IsComment() bool {
	return t.CommentID != ""
}

// Validate reports whether the target names something that could exist. It borrows the post's and
// the comment's own setters rather than restating their rules, so a target can only address what
// those two can be stored at.
func (t ReactionTarget) Validate() error {
	var post Blog
	if err := post.SetSlug(t.BlogSlug); err != nil {
		return err
	}
	if t.CommentID == "" {
		return nil
	}

	var comment Comment
	return comment.SetID(t.CommentID)
}

// Key is the target's half of a reaction's identity, and is what makes the two kinds distinguishable
// inside one collection: "post" for the post, "comment-{id}" for a comment on it. The post's slug
// is left out because it names the collection itself (see the repository), so repeating it here
// would only make every key longer by the same prefix.
//
// A comment id is letters and digits alone (see Comment.SetID), so the hyphen cannot be ambiguous
// and no comment's key can ever collide with the post's.
func (t ReactionTarget) Key() string {
	if !t.IsComment() {
		return "post"
	}
	return "comment-" + t.CommentID
}

// Reaction is one reader's reactions to one target: everything they chose, in one place, rather
// than a record per emoji.
//
// Grouping it by reader is what makes reacting cheap and safe at once. The document a reaction
// lives in is keyed by the target and the reader together (see Key), so two readers reacting to
// the same post write different documents and never contend, while one reader clicking twice
// contends only with themselves. The alternative - a document per (target, reader, emoji) - would
// make every read of a busy post fan out over far more documents to answer the same question.
type Reaction struct {
	Target ReactionTarget `json:"target"`
	// UID is the reader who reacted. It is never reported to clients: a reaction is shown as a
	// count and as whether *you* are in it (see the service's response), which is the whole of
	// what the bar renders and the least it can be answered with.
	UID       string    `json:"-"`
	Emojis    []string  `json:"emojis"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Key is where the reaction is stored beneath its post: the target it is on, and the reader who
// left it. Uniqueness of the pair is therefore a property of the document key rather than
// something checked on write - the same argument that keys a post by its slug and a profile by its
// account id.
func (r Reaction) Key() string {
	return r.Target.Key() + "-" + r.UID
}

// Validate reports whether the reaction is in a storable state: it names a target that could
// exist, a reader, and only emoji from AllowedEmojis, none twice. There is no separate bound on
// how many a reader may hold at once - AllowedEmojis is itself the bound, since none can repeat.
func (r Reaction) Validate() error {
	if err := r.Target.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.UID) == "" {
		return ValidationError{Field: "uid", Message: "is required"}
	}

	seen := make([]string, 0, len(r.Emojis))
	for _, emoji := range r.Emojis {
		if !ValidEmoji(emoji) {
			return ValidationError{Field: "emoji", Message: "must be one of the allowed reactions"}
		}
		if slices.Contains(seen, emoji) {
			return ValidationError{Field: "emoji", Message: "is already there"}
		}
		seen = append(seen, emoji)
	}
	return nil
}

// Add records one more emoji from this reader, and reports whether anything changed. Reacting with
// an emoji already chosen is not an error: the button that sends it is the same button whatever
// the stored state, so a second click from a stale page is answered with the state it wanted
// rather than with a complaint.
func (r *Reaction) Add(emoji string) (bool, error) {
	if !ValidEmoji(emoji) {
		return false, ValidationError{Field: "emoji", Message: "must be one of the allowed reactions"}
	}
	if slices.Contains(r.Emojis, emoji) {
		return false, nil
	}

	r.Emojis = append(r.Emojis, emoji)
	return true, nil
}

// Remove takes one emoji back, and reports whether anything changed. Like Add it is idempotent:
// removing one that was never there leaves the reader where they wanted to be.
func (r *Reaction) Remove(emoji string) bool {
	remaining := slices.DeleteFunc(slices.Clone(r.Emojis), func(each string) bool {
		return each == emoji
	})
	if len(remaining) == len(r.Emojis) {
		return false
	}

	r.Emojis = remaining
	return true
}

// IsEmpty reports whether the reader has nothing left on this target, which is what has the
// repository delete the record rather than store an empty one.
func (r Reaction) IsEmpty() bool {
	return len(r.Emojis) == 0
}
