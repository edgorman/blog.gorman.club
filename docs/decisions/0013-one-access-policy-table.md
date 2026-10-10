# 13. One access policy table, closed by default

Status: Accepted

## Context

A rule per feature drifts, and a feature added without one ends up open.

## Decision

Who may do what is one model (`services/backend/internal/entity/access.go`). Every resource (a post, a profile, a comment, a reaction, the assistant) declares an access mode for each action (`read`, `create`, `update`, `delete`). There are exactly three modes: public (everybody, signed in or not), private (the owner alone) and whitelist (the owner plus whoever is named). One table holds every pair, and a pair it doesn't declare is refused.

- A post is the one resource that picks its own read audience: its `visibility` and `allowedUserIds` are the three modes read back as one.
- The assistant is the one whitelist that isn't a field on a document; its membership is the subscription ([12](0012-assistant-entitlement-is-a-subscription.md)).
- There are no roles. Every rule is decided by who owns a thing, who was named on it, or what an account has paid for, and a role would sit between those rather than answer them.
- Comments and reactions are stored under their post and governed by the post's read rules, not rules of their own.

## Consequences

A new resource or action needs a line in the table, or it is refused. `services/backend/README.md` explains how a permission is asked and why a refused read is a `404`, not a `403`.
