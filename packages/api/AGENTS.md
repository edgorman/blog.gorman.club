# packages/api

The OpenAPI contract both services generate from. Reasoning for each rule is in [34](../../docs/decisions/0034-openapi-is-the-api-contract.md).

## Contract Layer

- `openapi.yaml` is the only place a route, parameter, body or status code is declared. Change it, run `moon run packages/api:generate`, and commit `services/backend/internal/api` and `services/frontend/src/gen` with it. Never hand-edit either; `api-drift` fails on any difference ([34](../../docs/decisions/0034-openapi-is-the-api-contract.md)).
- Pin every generator exactly: `oapi-codegen` as a `go tool` in `services/backend/go.mod` (its config is `services/backend/oapi-codegen.yaml`), and `openapi-typescript` in this directory's `package.json` and `openapi-fetch` in `services/frontend/package.json`, both without `^`. This directory has its own `package.json` because `openapi-typescript` needs the TypeScript 5 compiler API, which the frontend's TypeScript 7 does not ship.
- The spec models the wire only. Validation and policy stay in `internal/entity`, storage shapes in `internal/repository/firestore`, and Gemini's own types in `internal/repository/gemini/protocol.go`. One entity may need several schemas (`User` vs `CurrentUser`).
- Mark a property `required` when the server always sends it. Leave it out only where "absent" and "empty" mean different things (`UpdateCurrentUserRequest.username`, `ChatRequest.title`), which makes it a pointer in Go and optional in TypeScript. Build every list non-nil in the service, so an empty one is `[]`, never `null`.
- Every operation declares how it treats a credential, and the backend applies it: no `security` for a route that never looks at one, `security` with an extra empty requirement (`- {}`) for a read that admits anonymous callers but authorizes a signed-in one, and plain `security` for a route that needs a verified caller. `TestRouteAuthModes` lists them all; a new route needs a line there.
- Every non-2xx response is `ErrorResponse`: declare it as the `default` response through `#/components/responses/Error`.
- A closed set of values (`visibility`, `role`, moderation `status`) is an `enum`; the entity validates it too. The five reaction emoji are the exception: the contract says an emoji is a string, and the glyphs stay literals in `entity/emoji.go` and `ReactionBar.tsx`.
- Timestamps are `type: string, format: date-time`.
- No hand-written wire types: no `export interface` in `api.ts`, no `json:"..."` tags in `internal/service`. `root:wire-types` enforces both. Errors use `api.ErrorResponse`.
- Tasks: `generate` (both generators), `drift` (`api-drift`), and `breaking` (`oasdiff` against `origin/main`'s spec). The spec's linter is `TestSpec_IsValid` in `services/backend/internal/service`.

## A wire change is a two-release problem

- Make wire changes additive. `api-breaking` runs on every PR; for a deliberate break, mark the PR breaking (`!` or `BREAKING CHANGE:`), which skips the check and bumps major ([34](../../docs/decisions/0034-openapi-is-the-api-contract.md)).
- Check anything larger than an added property against the release order: the backend deploys before the frontend, so a new backend serves the previous bundle for a short window.
