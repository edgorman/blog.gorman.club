// Package worker receives the Firestore document events Eventarc delivers to the worker service.
//
// Each event arrives as a binary-mode CloudEvent: its attributes are ce-* headers and its data is
// a JSON DocumentEventData, because the triggers set event_data_content_type to application/json
// (see infrastructure/env/worker.tf). That keeps decoding to encoding/json, with no CloudEvents or
// protobuf dependency for a body this small.
//
// Eventarc retries any non-2xx answer, so the status code is a decision, not a report: 2xx once
// an event is handled or deliberately ignored, 5xx only when trying again could succeed.
package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// Document is the part of a Firestore document the worker reads. Fields stays raw until a handler
// needs it: its values are Firestore's typed wire form ({"stringValue": ...}), not plain JSON.
type Document struct {
	Name   string          `json:"name"`
	Fields json.RawMessage `json:"fields,omitempty"`
}

// Event is one Firestore document event. Value is nil when the document was deleted and OldValue
// is nil when it was created.
type Event struct {
	// Type is the CloudEvent type, e.g. google.cloud.firestore.document.v1.written.
	Type string `json:"-"`
	// Document is the path within the database, e.g. blogs/hello-world.
	Document string    `json:"-"`
	Value    *Document `json:"value"`
	OldValue *Document `json:"oldValue"`
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
		// A body that doesn't decode now never will, so it is acknowledged rather than retried.
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			log.ErrorContext(r.Context(), "event dropped: undecodable body", "type", e.Type, "document", e.Document, "error", err)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := h(r.Context(), e); err != nil {
			log.ErrorContext(r.Context(), "event failed, will be retried", "type", e.Type, "document", e.Document, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
