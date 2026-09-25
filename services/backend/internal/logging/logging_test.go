package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestNewHandler_WritesCloudLoggingFields(t *testing.T) {
	for _, tc := range []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARNING"},
		{slog.LevelError, "ERROR"},
		{slog.LevelError + 4, "ERROR"},
	} {
		var out bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{
			Level:       slog.LevelDebug,
			ReplaceAttr: cloudLoggingAttr,
		}))
		logger.Log(t.Context(), tc.level, "hello", "outcome", "ok")

		var entry map[string]any
		if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
			t.Fatalf("level %v: not a JSON line: %q", tc.level, out.String())
		}
		if entry["severity"] != tc.want {
			t.Errorf("level %v: severity = %v, want %s", tc.level, entry["severity"], tc.want)
		}
		if entry["message"] != "hello" {
			t.Errorf("level %v: message = %v, want hello", tc.level, entry["message"])
		}
		if entry["outcome"] != "ok" {
			t.Errorf("level %v: outcome = %v, want ok", tc.level, entry["outcome"])
		}
		for _, key := range []string{"level", "msg"} {
			if _, ok := entry[key]; ok {
				t.Errorf("level %v: %q still present, Cloud Logging would not read it", tc.level, key)
			}
		}
	}
}

func TestNewHandler_LeavesGroupedFieldsAlone(t *testing.T) {
	var out bytes.Buffer
	slog.New(NewHandler(&out)).Info("hello", slog.Group("request", "level", "caller data"))

	var entry struct {
		Request map[string]any `json:"request"`
	}
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatalf("not a JSON line: %q", out.String())
	}
	if entry.Request["level"] != "caller data" {
		t.Errorf("grouped level = %v, want it untouched", entry.Request["level"])
	}
}

// An attribute a caller names "level" is theirs, not the entry's severity, and must not be mistaken
// for one.
func TestNewHandler_CallerLevelAttribute(t *testing.T) {
	var out bytes.Buffer
	slog.New(NewHandler(&out)).Info("hello", "level", 3)

	var entry map[string]any
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatalf("not a JSON line: %q", out.String())
	}
	if entry["severity"] != "INFO" {
		t.Errorf("severity = %v, want INFO", entry["severity"])
	}
}
