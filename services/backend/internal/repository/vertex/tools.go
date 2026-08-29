package vertex

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// The tools the model is given. They are the whole of what it can do: change the title, change the
// body, or swap a passage for another. There is no tool for publishing, for visibility, for
// sharing, and none for reading or writing any post other than the one being discussed - so
// "automatically updating the contents" is bounded by this list rather than by the model's
// judgement.
const (
	toolSetTitle    = "set_title"
	toolSetContent  = "set_content"
	toolReplaceText = "replace_text"
)

// summaryExcerpt is how much of a value a ChatEdit's summary quotes back, in runes. It is short
// because the summary is a line in a chat transcript, not a diff.
const summaryExcerpt = 60

var declarations = []functionDeclaration{
	{
		Name:        toolSetTitle,
		Description: "Set the post's title. Use this whenever the title should change, including when the user asks for title suggestions and picks one.",
		Parameters: schema{
			Type: "OBJECT",
			Properties: map[string]schema{
				"title": {Type: "STRING", Description: "The new title, as plain text with no markdown heading marker."},
			},
			Required: []string{"title"},
		},
	},
	{
		Name:        toolSetContent,
		Description: "Replace the post's entire markdown body. Use this for a rewrite or a substantial restructure; prefer replace_text for a small, local change.",
		Parameters: schema{
			Type: "OBJECT",
			Properties: map[string]schema{
				"content": {Type: "STRING", Description: "The complete new body of the post, in markdown."},
			},
			Required: []string{"content"},
		},
	},
	{
		Name:        toolReplaceText,
		Description: "Replace every occurrence of an exact passage in the post's body. Use this for small, local edits such as fixing a typo or rewording a sentence. The passage must match the current body exactly.",
		Parameters: schema{
			Type: "OBJECT",
			Properties: map[string]schema{
				"find":    {Type: "STRING", Description: "The exact text to replace, copied verbatim from the current body."},
				"replace": {Type: "STRING", Description: "The text to put in its place. Use an empty string to delete the passage."},
			},
			Required: []string{"find", "replace"},
		},
	},
}

// apply runs one tool call against the draft and returns what to tell the model. A call that fails
// - unknown tool, unparseable arguments, a passage that is not in the post, a title over the
// length limit - returns a zero ChatEdit and an explanation, which goes back as the tool's result
// so the model can correct itself. Nothing here returns an error: a bad call is a turn in the
// conversation, not a failed request.
func apply(draft *entity.Draft, call functionCall) (entity.ChatEdit, string) {
	var args struct {
		Title   string  `json:"title"`
		Content string  `json:"content"`
		Find    string  `json:"find"`
		Replace *string `json:"replace"`
	}
	if len(call.Args) > 0 {
		if err := json.Unmarshal(call.Args, &args); err != nil {
			return entity.ChatEdit{}, fmt.Sprintf("error: arguments could not be read: %v", err)
		}
	}

	switch call.Name {
	case toolSetTitle:
		previous := draft.Title
		if err := draft.SetTitle(args.Title); err != nil {
			return entity.ChatEdit{}, "error: " + err.Error()
		}
		if draft.Title == previous {
			return entity.ChatEdit{}, "no change: the title already reads that way"
		}
		return edit(toolSetTitle, fmt.Sprintf("Set the title to %q", excerpt(draft.Title))), "ok: the title is now " + draft.Title

	case toolSetContent:
		previous := draft.Content
		if err := draft.SetContent(args.Content); err != nil {
			return entity.ChatEdit{}, "error: " + err.Error()
		}
		if draft.Content == previous {
			return entity.ChatEdit{}, "no change: the post already reads that way"
		}
		return edit(toolSetContent, fmt.Sprintf("Rewrote the post (%d characters)", len([]rune(draft.Content)))), "ok: the post was replaced"

	case toolReplaceText:
		replacement := ""
		if args.Replace != nil {
			replacement = *args.Replace
		}
		if err := draft.ReplaceText(args.Find, replacement); err != nil {
			return entity.ChatEdit{}, "error: " + err.Error()
		}
		summary := fmt.Sprintf("Replaced %q with %q", excerpt(args.Find), excerpt(replacement))
		if replacement == "" {
			summary = fmt.Sprintf("Deleted %q", excerpt(args.Find))
		}
		return edit(toolReplaceText, summary), "ok: the passage was replaced"

	default:
		return entity.ChatEdit{}, fmt.Sprintf("error: %q is not a tool this post has", call.Name)
	}
}

func edit(tool, summary string) entity.ChatEdit {
	return entity.ChatEdit{Tool: tool, Summary: summary}
}

// excerpt shortens a value for a summary line and flattens the newlines out of it, so a summary
// stays one line however many the passage ran to.
func excerpt(value string) string {
	flattened := strings.Join(strings.Fields(value), " ")
	return truncate(flattened, summaryExcerpt)
}
