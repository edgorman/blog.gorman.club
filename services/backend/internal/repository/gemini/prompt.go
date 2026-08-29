package gemini

import (
	"fmt"
	"strings"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// instructions is the system prompt, rebuilt on every round so it always carries the draft as it
// stands rather than as it was when the turn started. The post is given here rather than as a
// conversation turn for two reasons: it is state, not something anyone said, and a stored history
// that quoted the post would go stale the moment the post changed.
func instructions(draft entity.Draft) string {
	title := draft.Title
	if title == "" {
		title = "(untitled)"
	}
	content := draft.Content
	if strings.TrimSpace(content) == "" {
		content = "(the post is empty)"
	}

	return fmt.Sprintf(`You are a writing assistant built into the editor of a personal blog. You are helping the author of one specific post improve it.

How to work:
- Make changes by calling the tools. Never paste a rewritten post into your reply and never ask the author to copy anything: they see the post update in the editor as you edit it.
- Prefer %s for small, local changes; use %s only for a rewrite or a substantial restructure.
- Make the change the author asked for and nothing more. Do not reorganise, retitle, or "improve" parts they did not mention.
- If the request is ambiguous enough that you would be guessing, ask instead of editing.
- Answer questions about the post without editing it. Not every message is an instruction.

How to reply:
- Reply in plain prose, at most two or three sentences, saying what you did or answering what was asked.
- The post is written in markdown; keep the author's voice, formatting conventions, and heading structure.
- You are talking to the author about their own writing. Be direct and concise, and skip the preamble.

The post as it currently stands:

Title: %s

Body:
"""
%s
"""`, toolReplaceText, toolSetContent, title, content)
}

// conversation turns the stored history and the new message into the turns the API expects. Edits
// are replayed as text on the assistant's turn, so a model reading back the conversation can see
// that "make it shorter" was already acted on - a silent turn would read as a request nobody
// answered.
func conversation(req repository.AssistantRequest) []content {
	contents := make([]content, 0, len(req.History)+1)

	for _, message := range req.History {
		text := message.Content
		if len(message.Edits) > 0 {
			var summaries []string
			for _, made := range message.Edits {
				summaries = append(summaries, "- "+made.Summary)
			}
			text = strings.TrimSpace(text + "\n\nEdits made:\n" + strings.Join(summaries, "\n"))
		}
		if text == "" {
			continue
		}

		role := roleUser
		if message.Role == entity.ChatRoleAssistant {
			role = roleModel
		}
		contents = append(contents, content{Role: role, Parts: []part{{Text: text}}})
	}

	return append(contents, content{Role: roleUser, Parts: []part{{Text: req.Message}}})
}
