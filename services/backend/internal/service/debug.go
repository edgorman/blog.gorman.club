package service

import (
	"net/http"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/api"
)

// debugResponse is the Debug Endpoint Contract (`docs/decisions/0031-debug-endpoint-contract.md`):
// the generated api.DebugResponse, which the GCP uptime check matches on by the literal
// `"status":"ok"` substring.
func (s *Service) debugResponse() api.DebugResponse {
	return api.DebugResponse{
		Status:      "ok",
		Timestamp:   time.Now().UTC().Truncate(time.Second),
		Environment: s.cfg.Environment,
		Commit:      s.cfg.Commit,
	}
}

// GetDebug serves /debug.
func (s *Service) GetDebug(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.debugResponse())
}

// GetHealth serves /health, with the same answer as /debug.
func (s *Service) GetHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.debugResponse())
}
