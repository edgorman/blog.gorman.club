# services/backend

The Go backend on Cloud Run, plus the event worker. `README.md` here has the API, configuration and how to run it locally. Reasoning for each rule is in the linked record under `docs/decisions/`.

## AI Writing Assistant

- Call Gemini on the Agent Platform (`aiplatform.googleapis.com`) with Application Default Credentials. Never add an API key or use `generativelanguage.googleapis.com` ([11](../../docs/decisions/0011-assistant-on-agent-platform.md)).
- Model ids and locations are Terraform variables (`assistant_model`, `assistant_location`, and the worker's `embedding_*`, `moderation_*`), not constants ([11](../../docs/decisions/0011-assistant-on-agent-platform.md)).
- Entitlement is `subscribedUntil` on the account's profile, looked up by the verified uid, and nothing else: no allowlist, never a username or address ([12](../../docs/decisions/0012-assistant-entitlement-is-a-subscription.md)).
- Rate limit assistant turns per account, tighter than any other route ([12](../../docs/decisions/0012-assistant-entitlement-is-a-subscription.md)).

## Access Control

- Every resource and action is a line in `internal/entity/access.go`'s policy table, with one of three modes: public, private, whitelist. An undeclared pair is refused ([13](../../docs/decisions/0013-one-access-policy-table.md)).
- No roles ([13](../../docs/decisions/0013-one-access-policy-table.md)).
- A refused read is a `404`, not a `403` (see `README.md`).

## Reader Engagement

- Comments and reactions are stored under their post and follow the post's read rules ([13](../../docs/decisions/0013-one-access-policy-table.md)).
- Reactions are the five `entity.AllowedEmojis`, addressed with `PUT`/`DELETE`, never toggled ([8](../../docs/decisions/0008-allowed-emojis-stay-literals.md)).

## Finding Posts

- `author`, `tag` and `q` only narrow the feed; check `CanBeReadBy` on every candidate a search or related-posts lookup returns ([14](../../docs/decisions/0014-filters-narrow-the-feed.md)).
- Normalize tags on the way in (lowercase, hyphenated) ([14](../../docs/decisions/0014-filters-narrow-the-feed.md)).
- Search falls back to the substring scan rather than failing, and is one page with `hasMore: false` ([14](../../docs/decisions/0014-filters-narrow-the-feed.md)).
- `q` has its own rate-limit bucket (`searchesPerClient`) ([14](../../docs/decisions/0014-filters-narrow-the-feed.md)).

## Caching

- Cache only the anonymous `GET /blogs` pages, as a repository decorator (`internal/repository/cache`). Never cache a signed-in caller's page ([15](../../docs/decisions/0015-cache-only-the-anonymous-feed.md)).
- Every filter is part of the key, every write drops the cache, and entries are capped ([15](../../docs/decisions/0015-cache-only-the-anonymous-feed.md)).
- Rate-limit buckets and the cache are per instance: revisit both before running more than one instance ([12](../../docs/decisions/0012-assistant-entitlement-is-a-subscription.md)).

## Container image

- `services/backend:image` builds the Dockerfile with `COMMIT_SHA`; CI builds it on every PR but only publishes on merge ([19](../../docs/decisions/0019-hand-declared-moon-graph.md)).

## Worker

- Work that follows a Firestore write goes in `cmd/worker` with handlers in `internal/worker`, in this module ([16](../../docs/decisions/0016-worker-is-a-second-binary.md)).
- Answer 500 only when a retry could succeed, 204 otherwise. Handlers must be idempotent and a no-op when nothing relevant changed ([16](../../docs/decisions/0016-worker-is-a-second-binary.md)).
- Read the document from Firestore; don't decode the event body ([16](../../docs/decisions/0016-worker-is-a-second-binary.md)).
- Never write `blogs/` from the worker; embeddings go in `embeddings/{slug}` ([16](../../docs/decisions/0016-worker-is-a-second-binary.md)).
- The worker stays internal-only, invoked only by `worker-trigger` ([16](../../docs/decisions/0016-worker-is-a-second-binary.md)).
- Log one JSON line per event with `message` and `severity` ([16](../../docs/decisions/0016-worker-is-a-second-binary.md)).
- Moderate comments after publishing. Send the comment as data, never in the instructions, and treat anything but a well-formed verdict as a retry ([17](../../docs/decisions/0017-comments-published-then-moderated.md)).
