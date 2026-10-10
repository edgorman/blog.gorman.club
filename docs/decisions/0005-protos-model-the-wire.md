# 5. Protos model the wire, not the domain or the datastore

Status: Accepted

## Context

Three representations already existed when protos arrived (#168): `internal/entity` (validation and policy, e.g. `SetTitle`, `Permission`, `Subscribed`), `internal/repository/firestore`'s `...Document` structs with converters, and `internal/service`, the wire.

## Decision

`packages/protos` defines the wire and nothing else. Entities and Firestore documents stay hand-written.

- One entity can need several wire messages. `blogv1.User` has no `subscribedUntil` because a public lookup must never disclose it; `blogv1.CurrentUser` has it, because an account may see its own.
- Wire messages carry what the server resolves: `Blog.author_username` and `Comment.author_username` have no counterpart on the entity.
- Aggregates are wire-only: `ReactionCount` and `PageReactions` are folded from stored `entity.Reaction` rows (`countReactions`), with no entity to mirror.
- `protojson` can only marshal a message at the top level, and a map value must be a message, so lists get one-field wrappers: `TargetReactions`, `CommentThread`, `ChatHistory`.
- A field where "absent" and "empty" mean different requests is `optional`: `ChatRequest.title`/`content` (omitted means "use the saved post", `""` means the author cleared it), like `ListBlogsParams`.
- Reactions are addressed in the URL (`PUT`/`DELETE .../reactions/{emoji}`), so those routes read no request message.
- `chat.proto` models only the browser-to-backend hop. `internal/repository/gemini/protocol.go` stays hand-written: it describes Google's API, which this repo doesn't define or version.
- Entitlement and rate limiting stay out of the protos: a proto models what is exchanged, not who may ask.

## Consequences

Generated structs can't replace entities anyway: they carry `protoimpl` state Firestore would try to persist, and none of the entity methods.
