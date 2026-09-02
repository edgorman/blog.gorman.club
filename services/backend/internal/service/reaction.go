package service

import (
	"cmp"
	"net/http"
	"slices"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// reactionCount is one emoji as a bar renders it: the glyph, how many readers chose it, and
// whether the caller is one of them.
//
// Who reacted is deliberately not reported. A count and a "you are in it" is the whole of what the
// bar draws, and naming the readers would turn a one-click gesture into a public record of who
// liked what - a heavier thing than the button suggests, and not one a reader opts into by
// clicking.
type reactionCount struct {
	Emoji   string `json:"emoji"`
	Count   int    `json:"count"`
	Reacted bool   `json:"reacted"`
}

// reactionsResponse is every reaction on a page: the post's, and each commented-on comment's by
// id. Comments with no reactions are absent rather than present and empty, so the map holds only
// what there is something to draw for.
type reactionsResponse struct {
	Post     []reactionCount            `json:"post"`
	Comments map[string][]reactionCount `json:"comments"`
}

// countReactions folds the readers' rows into the counts a bar is drawn from. Ordering is by count
// and then by the emoji itself rather than by when each first appeared: a bar that reorders itself
// as people click is hard to aim at, and "most chosen first" is stable for everyone reading it.
func countReactions(reactions []entity.Reaction, uid string) []reactionCount {
	counts := make(map[string]*reactionCount)
	for _, reaction := range reactions {
		for _, emoji := range reaction.Emojis {
			count, seen := counts[emoji]
			if !seen {
				count = &reactionCount{Emoji: emoji}
				counts[emoji] = count
			}
			count.Count++
			// The zero uid is nobody, so a signed-out reader is never "in" a reaction - even
			// though a reaction with no reader could not have been stored in the first place.
			if uid != "" && reaction.UID == uid {
				count.Reacted = true
			}
		}
	}

	ordered := make([]reactionCount, 0, len(counts))
	for _, count := range counts {
		ordered = append(ordered, *count)
	}
	slices.SortFunc(ordered, func(a, b reactionCount) int {
		if byCount := cmp.Compare(b.Count, a.Count); byCount != 0 {
			return byCount
		}
		return cmp.Compare(a.Emoji, b.Emoji)
	})
	return ordered
}

// forTarget narrows a post's reactions to the ones on a single target.
func forTarget(reactions []entity.Reaction, target entity.ReactionTarget) []entity.Reaction {
	matching := make([]entity.Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		if reaction.Target == target {
			matching = append(matching, reaction)
		}
	}
	return matching
}

// GetReactions returns every reaction on a post and on its comments, in one response.
//
// It is one route rather than a reactions field on the post and on each comment because it is one
// query: the reactions to a post and to everything under it live together (see the repository), so
// splitting them across the responses that carry a post and its comments would mean reading them
// twice to say the same thing. A reader who may read the post may read them; one who may not gets
// the post's own 404.
func (s *Service) GetReactions(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}

	reactions, err := s.reactions.List(r.Context(), blog.Slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	uid := uidFromContext(r.Context())
	response := reactionsResponse{
		Post:     countReactions(forTarget(reactions, entity.PostReaction(blog.Slug)), uid),
		Comments: make(map[string][]reactionCount),
	}
	for _, reaction := range reactions {
		if !reaction.Target.IsComment() {
			continue
		}
		if _, counted := response.Comments[reaction.Target.CommentID]; counted {
			continue
		}
		response.Comments[reaction.Target.CommentID] = countReactions(forTarget(reactions, reaction.Target), uid)
	}

	writeJSON(w, http.StatusOK, response)
}

// reactionTargetFromPath resolves what a reaction route addresses, along with the emoji it names.
// A comment target is checked to exist rather than taken from the path on trust: a reaction to a
// comment nobody wrote would be counted by nothing and shown nowhere, and would outlive every
// cleanup that runs when a comment is deleted.
func (s *Service) reactionTargetFromPath(w http.ResponseWriter, r *http.Request, blog entity.Blog) (entity.ReactionTarget, string, bool) {
	emoji := r.PathValue("emoji")
	if !entity.ValidEmoji(emoji) {
		writeError(w, http.StatusBadRequest, "emoji must be one of the allowed reactions")
		return entity.ReactionTarget{}, "", false
	}

	if r.PathValue("id") == "" {
		return entity.PostReaction(blog.Slug), emoji, true
	}

	comment, ok := s.commentFromPath(w, r, blog)
	if !ok {
		return entity.ReactionTarget{}, "", false
	}
	return entity.CommentReaction(blog.Slug, comment.ID), emoji, true
}

// writeTargetReactions answers a write with the target's counts as they now stand, rather than
// with the caller's own row or with nothing at all. A bar is a shared count, so the client that
// just changed it needs what everybody else's clicks have made of it - which an optimistic +1
// cannot know.
func (s *Service) writeTargetReactions(w http.ResponseWriter, r *http.Request, target entity.ReactionTarget) {
	reactions, err := s.reactions.List(r.Context(), target.BlogSlug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, countReactions(forTarget(reactions, target), uidFromContext(r.Context())))
}

// PutReaction adds the caller's reaction to a post or to one of its comments. It is a PUT rather
// than a toggling POST so that the same request always means the same thing: a client whose page
// is out of date, or whose click was retried, ends up where it was aiming rather than back where
// it started.
//
// Anyone who may read the post may react to it, its author included, and reacting needs a
// credential for the same reason commenting does - a reaction is one reader, counted once, which
// there is no way to mean without knowing who they are.
func (s *Service) PutReaction(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}
	target, emoji, ok := s.reactionTargetFromPath(w, r, blog)
	if !ok {
		return
	}

	// The row about to be written is the caller's own: a reaction is keyed by the target and the
	// reader together, and the reader half comes from the credential rather than from the request,
	// so there is no other reader's row to reach. What the permission still says is that reacting
	// takes an account, which requireAuth has already seen to.
	reaction := entity.Reaction{Target: target, UID: uidFromContext(r.Context())}
	if !requirePermission(w, r, reaction.Permission(entity.ActionCreate)) {
		return
	}

	if _, err := s.reactions.Add(r.Context(), target, reaction.UID, emoji); err != nil {
		// entity.Reaction.Add only fails on an emoji outside entity.AllowedEmojis, which
		// reactionTargetFromPath already refused - this is defense in depth, not a live path.
		writeValidationError(w, err)
		return
	}

	s.writeTargetReactions(w, r, target)
}

// DeleteReaction takes the caller's reaction back. Like PutReaction it says what the caller wants
// to be true rather than asking for a flip, so removing one that is already gone succeeds.
func (s *Service) DeleteReaction(w http.ResponseWriter, r *http.Request) {
	blog, ok := s.requireReadableBlog(w, r)
	if !ok {
		return
	}
	target, emoji, ok := s.reactionTargetFromPath(w, r, blog)
	if !ok {
		return
	}

	reaction := entity.Reaction{Target: target, UID: uidFromContext(r.Context())}
	if !requirePermission(w, r, reaction.Permission(entity.ActionDelete)) {
		return
	}

	if _, err := s.reactions.Remove(r.Context(), target, reaction.UID, emoji); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.writeTargetReactions(w, r, target)
}
