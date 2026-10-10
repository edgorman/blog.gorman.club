# 8. The allowed emoji stay as literals on both sides

Status: Accepted

## Context

`blog.v1.AllowedEmojis` was the first message in `packages/protos` and proved the pipeline. It mirrors `entity.AllowedEmojis` and is used only as a type (`AllowedEmojis['emoji']` in `ReactionBar.tsx`). #171 asked whether the five glyphs themselves should move into the proto so both sides read one source.

## Decision

Leave them as literals, permanently:

1. proto3 has no default for a scalar list that both `protoc-gen-go` and ts-proto read back as the same constant without a custom option plus runtime reflection on both sides: a mechanism to build and maintain.
2. The glyphs are policy, exactly what `entity.ValidEmoji` decides, and protos model the wire, not policy ([5](0005-protos-model-the-wire.md)).

`entity/emoji.go` and `ReactionBar.tsx` each keep their own list, typed against `genblogv1.AllowedEmojis.Emoji` / `AllowedEmojis['emoji']`.

## Consequences

A rename or resize of the field breaks both builds, but the two lists' values can drift. That is the accepted trade.
