# 17. Comments are published first and moderated afterwards

Status: Accepted

## Context

Comments are screened by a model. Holding each comment until the model answers would make a model outage stop all comments.

## Decision

- Comments publish immediately. On each new comment the worker ([16](0016-worker-is-a-second-binary.md), `internal/worker/moderation.go`) reads it back, skips it if it already has `moderation`, classifies it with `gemini.Moderator` (structured JSON output at temperature 0, the comment JSON-encoded inside a `<comment>` data block and never in the instructions), and writes `moderation: {status, category, model, at}`.
- Anything but a well-formed verdict is a 500 and is retried, never read as approval.
- `GET /blogs/{slug}/comments` drops flagged comments for everyone except their author (who sees them unmarked) and the post's owner (who alone is sent `moderation`). `PUT .../comments/{id}/moderation` lets the owner approve one.
- The model is `moderation_model`/`moderation_location` in Terraform.

## Consequences

A model outage leaves comments unscreened rather than unposted.
