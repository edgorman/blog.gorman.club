package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// debugResponse is the Debug Endpoint Contract from CLAUDE.md; mirrors the frontend's HealthStatus type field for field.
type debugResponse struct {
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	Environment string `json:"environment"`
	Commit      string `json:"commit"`
}

// newDebugHandler returns the shared handler for /health and /debug.
// environment comes from a Cloud Run env var (the image is promoted unmodified between environments); commit is baked in at build time.
func newDebugHandler(environment, commit string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := debugResponse{
			Status:      "ok",
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			Environment: environment,
			Commit:      commit,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}
}
