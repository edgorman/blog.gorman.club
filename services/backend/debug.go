package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// debugResponse is the Debug Endpoint Contract described in the repository
// root CLAUDE.md, and mirrors the frontend's HealthStatus type
// (services/frontend/src/lib/health.ts) field for field.
type debugResponse struct {
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	Environment string `json:"environment"`
	Commit      string `json:"commit"`
}

// newDebugHandler returns the shared handler for /health and /debug.
//
// environment and commit are captured once at startup rather than read from
// globals on every request: environment comes from a Cloud Run env var (the
// same image is promoted unmodified from staging to prod, per CLAUDE.md, so
// it can't be baked in at build time), while commit is baked in at build
// time via the `commit` linker variable in main.go, since the image is never
// rebuilt.
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
