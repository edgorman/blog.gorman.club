// Package worker receives the Firestore document events Eventarc delivers to the worker service.
//
// Each event arrives as a binary-mode CloudEvent: its attributes are ce-* headers and its data is
// a protobuf DocumentEventData, the only encoding Eventarc offers for Firestore events. The body is
// not read: the ce-type and ce-document headers say what happened to which document, and a
// handler that needs the document's contents reads it from Firestore, which also gives it the
// current version rather than the one at the time of the event.
//
// Eventarc retries any non-2xx answer, so the status code is a decision, not a report: 2xx once
// an event is handled or deliberately ignored, 5xx only when trying again could succeed.
package worker

import (
	"context"
	"log/slog"
	"net/http"
)

// Event is one Firestore document event.
type Event struct {
	// Type is the CloudEvent type, e.g. google.cloud.firestore.document.v1.written.
	Type string
	// Document is the path within the database, e.g. blogs/hello-world.
	Document string
}

// Handler handles one event. A returned error means the event should be delivered again.
type Handler func(context.Context, Event) error

// New returns the worker's routes, one per trigger. A nil blog handler only logs, for a
// deployment with no embedding model configured.
func New(log *slog.Logger, blog Handler) http.Handler {
	if blog == nil {
		blog = logOnly(log)
	}
	mux := http.NewServeMux()
	mux.Handle("POST /events/blog", serve(log, blog))
	mux.Handle("POST /events/comment", serve(log, logOnly(log)))
	return mux
}

// logOnly acknowledges an event and nothing else; the handlers that act on events replace it.
func logOnly(log *slog.Logger) Handler {
	return func(ctx context.Context, e Event) error {
		log.InfoContext(ctx, "event received", "type", e.Type, "document", e.Document)
		return nil
	}
}

func serve(log *slog.Logger, h Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e := Event{Type: r.Header.Get("Ce-Type"), Document: r.Header.Get("Ce-Document")}
		if err := h(r.Context(), e); err != nil {
			log.ErrorContext(r.Context(), "event failed, will be retried", "type", e.Type, "document", e.Document, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
