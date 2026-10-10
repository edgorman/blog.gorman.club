# 3. Every codegen tool is pinned exactly

Status: Accepted

## Context

The drift check ([2](0002-generated-code-is-committed.md)) compares regenerated output with what is committed. If any tool in the chain floated, the output could change on a PR that never touched a `.proto`, and that kind of noise gets a check disabled rather than fixed.

## Decision

- `buf` is pinned in the root `.prototools` (and matched by `.claude/hooks/session-start.sh`).
- `protoc-gen-go` runs as `go run google.golang.org/protobuf/cmd/protoc-gen-go@<version>` in `buf.gen.yaml`, the exact version `services/backend/go.mod` requires.
- `ts-proto` is a `services/frontend/package.json` devDependency without a `^`. `packages/protos` has no `package.json`, so `buf.gen.yaml` reaches into `services/frontend/node_modules/.bin/protoc-gen-ts_proto`.
- `@bufbuild/protobuf`, the runtime the generated code imports, is pinned exactly as a direct `dependencies` entry, since it ships in the production bundle.

## Consequences

A bump to `google.golang.org/protobuf` or `ts-proto` means bumping the matching pin, running `buf generate` and committing the output in the same PR.
