package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/api"
	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// maxRequestBody caps every body readProto reads. The largest legitimate one is a post at
// entity.MaxContentLength runes, which even at the 6 bytes a JSON-escaped control character takes
// stays well under this.
const maxRequestBody = 1 << 20

// readJSON decodes a request body into a generated request type, writing the error response and
// returning false if it is too large or malformed. A field the type does not know is ignored, so
// an additive change to the contract is safe to deploy before the client that sends it.
//
// It reads the body whole, so the cap is what stops a caller making a write route buffer whatever
// it sends.
func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBody))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		return false
	}
	if err == nil {
		err = json.Unmarshal(body, into)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError answers with the contract's one error envelope (api.ErrorResponse), shared across
// every route: every handler fails the same way regardless of what it was doing.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, api.ErrorResponse{Error: message})
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

// badRequest answers a query or path parameter the generated routes could not bind.
func badRequest(w http.ResponseWriter, _ *http.Request, err error) {
	var invalid *api.InvalidParamFormatError
	if errors.As(err, &invalid) {
		switch invalid.ParamName {
		case "limit":
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		case "startAfter":
			writeError(w, http.StatusBadRequest, "startAfter must be an RFC3339 timestamp")
			return
		}
	}
	writeError(w, http.StatusBadRequest, "invalid request")
}
