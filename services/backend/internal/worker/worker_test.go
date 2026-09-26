package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func httpRequest(path, body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
}

func serveRequest(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func post(h http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httpRequest(path, body)
	req.Header.Set("Ce-Type", "google.cloud.firestore.document.v1.written")
	req.Header.Set("Ce-Document", "blogs/hello-world")
	return serveRequest(h, req)
}

// Eventarc retries exactly the non-2xx answers, so this mapping is what decides redelivery.
func TestServeStatusMapping(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	ok := func(context.Context, Event) error { return nil }
	fail := func(context.Context, Event) error { return errors.New("firestore unavailable") }

	for _, tc := range []struct {
		name    string
		handler Handler
		body    string
		want    int
	}{
		// The body is protobuf in production and never read, so any bytes stand in for it.
		{"handled", ok, "\x0a\x02pb", http.StatusNoContent},
		{"retryable failure", fail, "\x0a\x02pb", http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := post(serve(quiet, tc.handler), "/", tc.body).Code; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestServeDecodesEvent(t *testing.T) {
	var got Event
	h := serve(slog.New(slog.NewTextHandler(io.Discard, nil)), func(_ context.Context, e Event) error { got = e; return nil })
	post(h, "/", "\x0a\x02pb")

	if got.Type != "google.cloud.firestore.document.v1.written" || got.Document != "blogs/hello-world" {
		t.Errorf("attributes = %q %q", got.Type, got.Document)
	}
}

func TestRoutes(t *testing.T) {
	var logged strings.Builder
	h := New(slog.New(slog.NewTextHandler(&logged, nil)), nil, nil)

	for _, path := range []string{"/events/blog", "/events/comment"} {
		if got := post(h, path, `{}`).Code; got != http.StatusNoContent {
			t.Errorf("POST %s = %d, want 204", path, got)
		}
	}
	if n := strings.Count(logged.String(), "event received"); n != 2 {
		t.Errorf("logged %d event lines, want 2:\n%s", n, logged.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/events/blog", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /events/blog = %d, want 405", rec.Code)
	}
}
