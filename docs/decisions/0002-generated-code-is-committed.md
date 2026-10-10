# 2. Generated code is committed, next to the code that uses it

Status: Accepted

## Context

An entity shape declared once in Go and hand-copied into TypeScript has nothing enforcing that the two agree: a renamed field is caught at runtime, if at all. `packages/protos` holds `.proto` definitions both sides generate from, built with buf. The generated code has to exist somewhere: `go mod tidy` and gopls need the generated packages on disk, not produced on demand by a build.

## Decision

- `services/backend/internal/gen` and `services/frontend/src/gen` are committed, ordinary first-party source.
- Generated code follows its consumer, not the contract it came from (the same rule as [1](0001-manifests-live-with-their-code.md)). `option go_package` points into `services/backend/internal/gen/...`; `buf.gen.yaml`'s `out:` puts the TypeScript into `services/frontend/src/gen`, not a shared `packages/node`.
- The `protos-drift` check (`packages/protos:drift`) re-runs `buf generate` and fails on `git status --porcelain` over `packages/protos` and both `gen/` folders. Its `inputs` cover the `gen/` folders too, so a hand-edit to generated output is caught, not only a `.proto` edit. It depends on `services/frontend:install`, because the ts-proto plugin is resolved from `services/frontend/node_modules/.bin`.
- `services/backend:test` and `services/frontend:test` list `project://packages/protos` as an input, so a `.proto`-only edit still marks both affected (see [19](0019-hand-declared-moon-graph.md)).

## Consequences

- Editors and Go tooling work with no extra step.
- Every `.proto` change must commit its regenerated output, and nobody edits `gen/` by hand.
