# packages/protos

The `.proto` contract shared by `services/backend` and `services/frontend`, built with buf. Reasoning for each rule is in the linked record under `docs/decisions/`.

## Contract Layer

- Change the `.proto`, run `buf generate` here, and commit `services/backend/internal/gen` and `services/frontend/src/gen` with it. Never hand-edit either; `protos-drift` fails on any difference ([2](../../docs/decisions/0002-generated-code-is-committed.md)).
- Generated code lives in the service that compiles it: `option go_package` points into `services/backend/internal/gen/...`, and `buf.gen.yaml` writes TypeScript to `services/frontend/src/gen` ([2](../../docs/decisions/0002-generated-code-is-committed.md)).
- Pin every tool exactly: `buf` in `.prototools`, `protoc-gen-go@<version>` in `buf.gen.yaml` matching `services/backend/go.mod`, and `ts-proto` and `@bufbuild/protobuf` in `services/frontend/package.json` without `^`. Bump a pin together with `buf generate` ([3](../../docs/decisions/0003-pin-every-codegen-tool.md)).
- Keep the ts-proto `opt` flags (`esModuleInterop`, `useOptionals=messages`, `forceLong=string`, `useDate=string`, `outputServices=false`) and `strategy: all`. Message-typed fields are optional, scalars are not; guard `createdAt?` and similar at call sites ([4](../../docs/decisions/0004-ts-proto-options.md)).
- Protos model the wire only. Validation and policy stay in `internal/entity`, storage shapes in `internal/repository/firestore`, and Gemini's own types in `internal/repository/gemini/protocol.go`. One entity may need several messages (`User` vs `CurrentUser`) ([5](../../docs/decisions/0005-protos-model-the-wire.md)).
- A list returned at the top level or as a map value needs a one-field wrapper message (`TargetReactions`, `CommentThread`, `ChatHistory`) ([5](../../docs/decisions/0005-protos-model-the-wire.md)).
- Use `optional` where "absent" and "empty" are different requests (`ChatRequest.title`) ([5](../../docs/decisions/0005-protos-model-the-wire.md)).
- Marshal with `protojson` via `internal/service/respond.go`, keeping `EmitDefaultValues`, lowerCamelCase names, `DiscardUnknown` on requests and compacted output ([6](../../docs/decisions/0006-protojson-settings.md)).
- Timestamps are `google.protobuf.Timestamp` ([4](../../docs/decisions/0004-ts-proto-options.md)).
- Closed sets of string values (`visibility`, `role`) stay `string`, not `enum`; validate them in the entity ([7](../../docs/decisions/0007-visibility-is-a-string.md)).
- `AllowedEmojis` shares the shape only; the five glyphs stay as literals in `entity/emoji.go` and `ReactionBar.tsx` ([8](../../docs/decisions/0008-allowed-emojis-stay-literals.md)).
- No hand-written wire types: no `export interface` in `api.ts`, no `json:"..."` tags in `internal/service` outside `debug.go`. `root:wire-types` enforces both. Errors use `blogv1.ErrorResponse` ([9](../../docs/decisions/0009-no-hand-written-wire-types.md)).
- Tasks: `lint` (`buf lint`), `format` (`buf format -d --exit-code`), `drift`, and `breaking` (`buf breaking` against `origin/main`).

## A wire change is a two-release problem

- Make wire changes additive. `protos-breaking` runs `buf breaking` on every PR; for a deliberate break, mark the PR breaking (`!` or `BREAKING CHANGE:`), which skips the check and bumps major ([10](../../docs/decisions/0010-wire-changes-across-releases.md)).
- Check anything larger than an added field against the release order: the backend deploys before the frontend ([10](../../docs/decisions/0010-wire-changes-across-releases.md)).
