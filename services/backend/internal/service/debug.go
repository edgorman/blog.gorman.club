package service

import (
	"net/http"
	"time"
)

// debugResponse is the Debug Endpoint Contract from CLAUDE.md, mirroring the frontend's
// HealthStatus type field for field.
type debugResponse struct {
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	Environment string `json:"environment"`
	Commit      string `json:"commit"`
}

// Debug serves both /health and /debug.
func (s *Service) Debug(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, debugResponse{
		Status:      "ok",
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		Environment: s.cfg.Environment,
		Commit:      s.cfg.Commit,
	})
}
