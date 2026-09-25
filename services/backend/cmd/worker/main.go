// Command worker handles the Firestore document events Eventarc delivers from the blog's
// collections. It is not publicly reachable: see infrastructure/env/worker.tf.
package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/worker"
)

// Baked in at build time via -ldflags (see Dockerfile); images are never rebuilt for production.
var commit = "unknown"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// JSON on stdout under the key names Cloud Logging reads, so each line is one structured entry
	// with its severity rather than a text payload.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.MessageKey:
				a.Key = "message"
			case slog.LevelKey:
				a.Key = "severity"
			}
			return a
		},
	})).With("commit", commit)

	// Timeouts for the same reason as cmd/backend's server; the worker has no long-running calls yet.
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           worker.New(logger),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	logger.Info("worker listening", "port", port)
	log.Fatal(server.ListenAndServe())
}
