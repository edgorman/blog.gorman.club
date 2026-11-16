# 34. OpenAPI is the API contract

Status: Accepted (supersedes [2](0002-generated-code-is-committed.md), [3](0003-pin-every-codegen-tool.md), [4](0004-ts-proto-options.md), [5](0005-protos-model-the-wire.md), [6](0006-protojson-settings.md), [7](0007-visibility-is-a-string.md), [8](0008-allowed-emojis-stay-literals.md), [9](0009-no-hand-written-wire-types.md) and [10](0010-wire-changes-across-releases.md))

## Context

The API is plain REST/JSON between one Go service and one TypeScript client. The `.proto` files described only message shapes: routes, methods, paths and the client were hand-written on both sides, and `protojson` was there to serialize messages that existed only to be serialized. Nothing checked that the frontend called a route the backend served, and the generated TypeScript made every message-typed field optional, so call sites guarded fields the server always sends.

## Decision

`packages/api/openapi.yaml` is the one contract, and both services generate from it.

- **Backend.** `oapi-codegen`, pinned as a `go tool` in `services/backend/go.mod`, writes `internal/api`: the models, a `net/http` `ServerInterface` registered on the `ServeMux`, and the spec embedded in the binary. A route exists exactly when the spec declares it and `service.Service` implements the operation; the compiler checks the second half.
- **Frontend.** `openapi-typescript` writes `src/gen/schema.d.ts`, and `openapi-fetch` makes typed calls. A path or method the spec does not declare fails `tsc`. `openapi-typescript` needs the TypeScript 5 compiler API, so its pin lives in `packages/api/package.json` rather than in the frontend, which runs TypeScript 7.
- **Required properties.** A property the server always sends is `required`, which gives a non-pointer Go field and a required TypeScript field. One that is not required is one where "absent" and "empty" differ (`PUT /users/me`'s `username`, `ChatRequest.title`), and is a pointer in Go. A list is never `null`: the service builds every list non-nil, so an empty one is `[]`.
- **Authentication** is declared per operation in `security` and read from the embedded spec when the service starts: no `security` means no credential is looked at, `security` with an empty requirement (`{}`) means a credential is verified if sent, and `security` alone means a verified caller is required and metered per account. A route cannot be added without saying which it is. The two routes that call a paid model are metered a second time inside their handlers (`ListBlogs` for a search, `SendChatMessage`), since that needs the caller the first step has resolved.
- **Closed sets.** `visibility`, a chat message's `role` and a comment's moderation `status` are `enum`s, so Go has typed constants and TypeScript a union. The entity still validates them. The five reaction emoji stay literals in `entity/emoji.go` and `ReactionBar.tsx`: the contract says an emoji is a string, since the glyphs are policy and not shape.
- **Bodies** are plain `encoding/json`. A field a type does not declare is ignored, which is what makes an additive change safe to deploy before the client that sends it. Timestamps are RFC 3339 `date-time`.
- **Generated code is committed**, next to its consumer, and nobody edits it by hand. `api-drift` (`packages/api:drift`) re-runs both generators and fails on `git status --porcelain` over `packages/api` and both generated outputs. Every generator is pinned exactly: `oapi-codegen` in `go.mod`, `openapi-typescript` and `openapi-fetch` without `^`.
- **No hand-written wire types.** `root:wire-types` fails on an `export interface` in `api.ts` and on any `json:"..."` tag in `internal/service`'s non-test files. What stays hand-written is authorization (`entity/access.go`), `Validate()`/`Set*`, storage shapes in `internal/repository/firestore`, Gemini's own types in `internal/repository/gemini/protocol.go`, and `config.ts`'s `RuntimeConfig` ([22](0022-build-once-promote-by-sha.md)).
- **Wire changes are additive.** A production release deploys the backend before the frontend ([22](0022-build-once-promote-by-sha.md)), so for a short window a new backend serves the previous bundle. `oasdiff` runs against `origin/main`'s spec as the required `api-breaking` check (`packages/api:breaking`, in `conventions.yaml`). A deliberate break is marked breaking (`!` in the PR title or a `BREAKING CHANGE:` footer), which skips the check and bumps the major version.
- **Validity.** `TestSpec_IsValid` loads the embedded spec and validates it as OpenAPI 3, which is the spec's linter; `TestRouteAuthModes` lists every route with its auth mode.

## Consequences

- A new resource starts as paths and schemas in `openapi.yaml`, then `moon run packages/api:generate`.
- `buf`, `protoc-gen-go`, `ts-proto` and the protobuf runtime are gone, and `internal/gen` and its TypeScript twin with them.
- Timestamps lose protojson's padding to 3, 6 or 9 fractional digits: `encoding/json` drops trailing zeros. Clients parse both, and the `startAfter` cursor round-trips.
- A request path with an empty segment (`/blogs//comments`) matches no route and gets the mux's plain 404 rather than the JSON envelope; nothing in the frontend builds one.
- A search or feed `limit` that is not a positive integer is still a `400` with the same message.
