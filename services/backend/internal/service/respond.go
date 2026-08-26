package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// apiError is the body of every non-2xx response, so clients parse success and failure the same way.
type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

// writeValidationError reports a failed entity rule as a 400 carrying the field and reason.
func writeValidationError(w http.ResponseWriter, err error) {
	var invalid entity.ValidationError
	if errors.As(err, &invalid) {
		writeError(w, http.StatusBadRequest, invalid.Error())
		return
	}
	writeError(w, http.StatusBadRequest, "invalid request body")
}
