# 6. protojson settings

Status: Accepted

## Context

`encoding/json` on a generated struct walks its Go fields, protoimpl state included, and ignores the `protobuf` tags that decide names, presence and well-known-type encoding. Responses are marshalled with `protojson` (`internal/service/respond.go`) instead. Its settings were chosen against captured response bodies from the `encoding/json` era, so the move changed nothing a client could see.

## Decision

- `EmitDefaultValues`, not `EmitUnpopulated`. Bare `protojson` drops zero scalars (`"assistantEnabled": false` would vanish); `EmitUnpopulated` emits `null` for unset message fields (`"subscribedUntil": null`). `EmitDefaultValues` matches the old behaviour.
- `UseProtoNames` off, so the wire stays lowerCamelCase (`createdAt`).
- `DiscardUnknown` on for requests. `protojson` rejects unknown fields by default; without this an older or newer client would get a `400`. It is also what makes an additive change safe to deploy before the client that sends it ([10](0010-wire-changes-across-releases.md)).
- Output is compacted before it is written (`writeProto`). `protojson` varies its whitespace between builds on purpose, and identical requests answering with byte-different bodies would defeat any caching or ETag added later.

## Consequences

These four settings are the whole compatibility story between the two encoders; changing one is a wire change.
