package gemini

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

func toolCall(t *testing.T, name string, args any) functionCall {
	t.Helper()

	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("encode args: %v", err)
	}
	return functionCall{Name: name, Args: encoded}
}

// A call that cannot be run is a turn in the conversation, not a failed request: nothing here
// returns an error, and the model is told enough to correct itself.
func TestApply_FailuresAreReportedToTheModel(t *testing.T) {
	for _, tt := range []struct {
		name string
		call functionCall
	}{
		{"unknown tool", toolCall(t, "delete_post", map[string]any{})},
		{"unreadable arguments", functionCall{Name: toolSetTitle, Args: json.RawMessage(`not json`)}},
		{"title over the limit", toolCall(t, toolSetTitle, map[string]any{"title": strings.Repeat("a", entity.MaxTitleLength+1)})},
		{"passage that is not there", toolCall(t, toolReplaceText, map[string]any{"find": "elephant", "replace": "x"})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			draft := entity.Draft{Title: "Hello", Content: "the cat sat"}

			edit, result := apply(&draft, tt.call)

			if edit.Tool != "" {
				t.Errorf("edit = %+v, want none recorded for a call that failed", edit)
			}
			if !strings.HasPrefix(result, "error:") {
				t.Errorf("result = %q, want it to report the error", result)
			}
			if (draft != entity.Draft{Title: "Hello", Content: "the cat sat"}) {
				t.Errorf("draft = %+v, want it untouched", draft)
			}
		})
	}
}

// An edit that changes nothing is not recorded, so the service is never told the post was rewritten
// when it was not.
func TestApply_NoOpIsNotAnEdit(t *testing.T) {
	draft := entity.Draft{Title: "Hello", Content: "the cat sat"}

	for _, call := range []functionCall{
		toolCall(t, toolSetTitle, map[string]any{"title": "Hello"}),
		toolCall(t, toolSetContent, map[string]any{"content": "the cat sat"}),
	} {
		edit, result := apply(&draft, call)

		if edit.Tool != "" {
			t.Errorf("%s recorded %+v, want no edit for a value that already reads that way", call.Name, edit)
		}
		if !strings.HasPrefix(result, "no change") {
			t.Errorf("result = %q, want it to say nothing changed", result)
		}
	}
}

func TestApply_Edits(t *testing.T) {
	draft := entity.Draft{Title: "Hello", Content: "the cat sat"}

	for _, tt := range []struct {
		call        functionCall
		wantTitle   string
		wantContent string
	}{
		{toolCall(t, toolSetTitle, map[string]any{"title": "  A better title  "}), "A better title", "the cat sat"},
		{toolCall(t, toolReplaceText, map[string]any{"find": "cat", "replace": "dog"}), "A better title", "the dog sat"},
		// An empty replacement deletes the passage rather than being read as a missing argument.
		{toolCall(t, toolReplaceText, map[string]any{"find": " sat", "replace": ""}), "A better title", "the dog"},
		{toolCall(t, toolSetContent, map[string]any{"content": "a whole new body"}), "A better title", "a whole new body"},
	} {
		t.Run(tt.call.Name, func(t *testing.T) {
			edit, result := apply(&draft, tt.call)

			if edit.Tool != tt.call.Name {
				t.Errorf("edit = %+v, want one recorded against %s", edit, tt.call.Name)
			}
			if edit.Summary == "" {
				t.Error("Summary is empty, want a line for the transcript")
			}
			if !strings.HasPrefix(result, "ok:") {
				t.Errorf("result = %q, want it to report success", result)
			}
			if draft.Title != tt.wantTitle || draft.Content != tt.wantContent {
				t.Errorf("draft = %+v, want {%q %q}", draft, tt.wantTitle, tt.wantContent)
			}
		})
	}
}

// A summary is a line in a transcript, so it stays one line however many the passage ran to.
func TestApply_SummaryIsOneShortLine(t *testing.T) {
	draft := entity.Draft{Content: "first line\nsecond line\n" + strings.Repeat("long ", 40)}

	edit, _ := apply(&draft, toolCall(t, toolReplaceText, map[string]any{
		"find":    "first line\nsecond line",
		"replace": strings.Repeat("long ", 40),
	}))

	if strings.Contains(edit.Summary, "\n") {
		t.Errorf("Summary = %q, want it flattened to one line", edit.Summary)
	}
	if len([]rune(edit.Summary)) > 2*summaryExcerpt+40 {
		t.Errorf("Summary is %d runes, want it kept short", len([]rune(edit.Summary)))
	}
}
