# 9. No hand-written wire types, enforced in CI

Status: Accepted

## Context

#173 swept `api.ts` and `internal/service` for anything #169-#172 left behind. The one real survivor was the `{"error": "..."}` envelope, hand-declared on the backend and mirrored inline in `api.ts`'s `request()`. It became `blogv1.ErrorResponse` (`error.proto`), one message shared by every resource. A dead `json:"authorUsername"` tag was removed. `entity.ValidationError` needed nothing: `writeValidationError` already collapses it to the one envelope.

## Decision

The `root:wire-types` moon task fails the PR on any `export interface` in `services/frontend/src/lib/api.ts`, or any `json:"..."` struct tag in `internal/service`'s non-test files other than `debug.go`. It runs in `lint-check`, a check already required, rather than a new required check that could merge red until someone added it to `settings.yml` (as happened to `protos-drift`, #154).

What stays hand-written, by design:

- `entity/access.go`'s policy table: authorization, not a shape.
- `internal/repository/gemini/protocol.go`: a third-party contract.
- Every `Validate()`/`Set*` method: business logic a proto can't express.
- `debug.go`'s `debugResponse`: nothing else declares that shape to drift against ([31](0031-debug-endpoint-contract.md)).
- `services/frontend/src/lib/config.ts`'s `RuntimeConfig`: `config.json` is written by the deploy pipeline, not served by the backend ([22](0022-build-once-promote-by-sha.md)).

## Consequences

A new wire type starts as a message in a `.proto`.
