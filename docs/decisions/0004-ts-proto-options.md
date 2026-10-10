# 4. ts-proto options

Status: Accepted

## Context

ts-proto's defaults don't match this frontend's compiler settings or what the wire actually carries.

## Decision

- `esModuleInterop=true`: `tsconfig.app.json`'s `verbatimModuleSyntax` rejects the default `import * as`.
- `useOptionals=messages`: only message-typed fields are optional; scalars and repeated fields stay required. This is what proto3 and `EmitDefaultValues` ([6](0006-protojson-settings.md)) guarantee together: a scalar is always on the wire, a message may be absent. Earlier docs said the flag made every field optional; #169 corrected that.
- `forceLong=string`: 64-bit integers decode to `string` rather than pulling in the `long` package.
- `useDate=string`: `google.protobuf.Timestamp` decodes to the RFC3339 string the wire carries, so Go keeps `timestamppb` and the generated type matches the response body without conversion.
- `outputServices=false`: there is no `service` in any `.proto`; the backend is plain REST.
- `strategy: all` on the plugin entry: ts-proto requires it under buf.

## Consequences

A timestamp is a message field, so it has presence: `createdAt?: string` where a hand-written interface said `createdAt: string`. That is the wire telling the truth (`protojson` may omit it), and call sites guard it.
