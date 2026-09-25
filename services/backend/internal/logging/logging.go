// Package logging writes the backend's logs in the shape Cloud Logging reads as structured.
//
// Cloud Run hands every line a container writes to stdout to Cloud Logging, and a line that is a
// JSON object is stored as a jsonPayload rather than as text: its fields can then be filtered on and
// extracted into log-based metrics (see infrastructure/env/monitoring.tf), which a plain line cannot.
// A few field names are special - "severity" and "message" among them - and are lifted out of the
// payload into the entry itself, so the only work here is to name slog's own keys the way Cloud
// Logging expects. Without it every line lands at DEFAULT severity and an error is
// indistinguishable from a startup notice.
package logging

import (
	"io"
	"log/slog"
)

// NewHandler returns a JSON handler writing to w, with slog's level and message keys renamed to
// the ones Cloud Logging recognizes.
func NewHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{ReplaceAttr: cloudLoggingAttr})
}

// cloudLoggingAttr renames the two top-level keys whose names differ. Only top-level attributes are
// touched: a group's own "level" or "msg" field is the caller's data, not the entry's.
func cloudLoggingAttr(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return attr
	}
	switch attr.Key {
	case slog.LevelKey:
		// The handler's own level is always a slog.Level; a caller's attribute that happens to be
		// named "level" is not, and is left alone rather than trusted to be one.
		if level, ok := attr.Value.Any().(slog.Level); ok {
			return slog.String("severity", severity(level))
		}
	case slog.MessageKey:
		attr.Key = "message"
	}
	return attr
}

// severity maps a slog level onto Cloud Logging's LogSeverity names. slog's WARN is the one whose
// name differs; a level between two of slog's own is rounded down to the one below it.
func severity(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARNING"
	case level >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}
