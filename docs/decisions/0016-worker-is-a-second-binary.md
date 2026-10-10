# 16. The event worker is a second binary in the backend module

Status: Accepted

## Context

Some work (embedding a post, moderating a comment) should run after a Firestore write rather than in the request that made it. It needs `internal/entity` and `internal/repository/firestore`, which Go's `internal/` rule keeps inside the backend module.

## Decision

- `services/backend/cmd/worker` is a second binary in the backend module, with handlers in `internal/worker`. It is its own moon project (`worker`) with only `worker:image`; format, vet and tests stay with `services/backend`'s `./...` tasks. The Dockerfile's `CMD` build arg picks the binary.
- Eventarc (`infrastructure/env/worker.tf`) delivers Firestore `written` events on `blogs/{slug}` to `POST /events/blog` and `created` events on comments to `POST /events/comment`. The body is protobuf `DocumentEventData` (Firestore offers no JSON; `event_data_content_type` must be `application/protobuf`, as `application/json` or unset makes trigger creation fail). The worker reads `Ce-Type` and `Ce-Document` and re-reads the document itself.
- Status codes decide retries: 500 only when trying again could succeed, 204 for a handled or ignored event. Handlers are idempotent and a no-op when nothing they care about changed, since the worker's own writes can fire triggers.
- It isn't public: internal-only ingress, and only `worker-trigger` holds `run.invoker`. It runs as `worker-runtime`.
- Embeddings live in `embeddings/{slug}` (vector, `contentHash`, model, `ownerId`), never in `blogs/`, so the worker doesn't fire its own trigger. A matching hash and model means no model call.
- On every start the worker syncs every post (bounded to two minutes, failures logged), so the first deploy backfilled old posts with no hand-run step.
- Logs are one JSON line per event (`message`/`severity`); an alert fires on a run of 500s.
- It ships like the backend ([22](0022-build-once-promote-by-sha.md)): `backend/worker:<sha>` to both registries on merge, the same image to prod on promotion.

## Consequences

Anything triggered by a write belongs here, idempotent and retry-safe.
