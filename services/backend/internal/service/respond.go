package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
	blogv1 "github.com/edgorman/blog.gorman.club/services/backend/internal/gen/blog/v1"
)

// protoJSON is how every generated message reaches the wire. The options are the whole of the
// compatibility story between encoding/json and protojson, so they are set once here rather than
// per call site:
//
//   - EmitDefaultValues keeps a zero-valued scalar in the body. protojson drops one by default,
//     which would silently turn `"assistantEnabled": false` into an absent field and a client's
//     `undefined`. Its narrower cousin EmitUnpopulated would go further and emit `null` for unset
//     message fields too, which would start sending `"subscribedUntil": null` where the field is
//     absent today - a change with no upside, so this is the one that matches encoding/json most
//     closely.
//   - UseProtoNames stays off (the default), so field names are lowerCamelCase on the wire -
//     `createdAt`, not `created_at` - which is what the hand-written `json` tags already produced
//     and what the frontend already reads.
//
// See `packages/protos/AGENTS.md`'s "Contract Layer" for why responses are built from generated messages at all.
var protoJSON = protojson.MarshalOptions{EmitDefaultValues: true}

// writeProto is writeJSON for a generated message. It exists because encoding/json cannot
// serialize one correctly: it would walk the struct's own fields - including protoimpl state - and
// ignore the `protobuf` tags that decide names, presence and well-known-type encoding.
func writeProto(w http.ResponseWriter, status int, message proto.Message) {
	body, err := protoJSON.Marshal(message)
	if err != nil {
		// A message this service built itself failing to marshal is a bug, not a client error, and
		// there is nothing useful to say about it beyond the status.
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// protojson deliberately varies its whitespace between builds, so the same message marshals to
	// `{"id":"x","bio":""}` in one binary and `{"id":"x", "bio":""}` in the next - upstream does
	// this to stop callers depending on exact bytes. Two identical requests answering with
	// byte-different bodies is a poor property for an HTTP API to have (it would quietly defeat
	// any response caching or ETag added later), and compacting costs one pass, so the wire is
	// normalized here instead.
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(compact.Bytes())
}

// protoJSONUnmarshal is the request-side counterpart. DiscardUnknown matters more than it looks:
// protojson rejects an unrecognized field by default, where encoding/json ignores one, so without
// it a client sending a field this version does not know - an older or newer frontend, or anything
// speculative - would start getting a 400 where it used to be accepted. Tolerating unknown fields
// is also what makes an additive proto change safe to deploy before the client that sends it.
var protoJSONUnmarshal = protojson.UnmarshalOptions{DiscardUnknown: true}

// readProto decodes a request body into a generated message, the counterpart to writeProto.
//
// It reads the body whole because protojson has no streaming decoder, where encoding/json did.
// That is not a new exposure in practice - decoding into a struct buffers the same bytes either
// way, and no route caps its body today - but a cap belongs here if one is ever wanted, rather
// than at each call site.
func readProto(r *http.Request, message proto.Message) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return protoJSONUnmarshal.Unmarshal(body, message)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeError answers with the wire's one error envelope (blogv1.ErrorResponse, #173), shared
// across every resource rather than declared per one: every handler fails the same way regardless
// of what it was doing, so there is exactly one shape to keep in sync with api.ts's own read of it.
func writeError(w http.ResponseWriter, status int, message string) {
	writeProto(w, status, &blogv1.ErrorResponse{Error: message})
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
