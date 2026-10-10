# 15. Cache only the anonymous feed

Status: Accepted

## Context

`GET /blogs` is the highest-traffic read, and the signed-out landing feed is the same public data for every visitor.

## Decision

Cache the anonymous caller's pages in the serving process for a short TTL (`internal/repository/cache`), as a decorator over the blog repository wired in `cmd/backend`, so the service can't tell a cached page from a fetched one and the cache is removed by deleting a line.

- Only the empty uid is cached. Two callers may share a page only if they share a uid, and the empty uid is granted public posts and nothing else. Signed-in pages are neither stored nor served, so there is no per-uid keying to get wrong.
- Every filter is part of the key.
- A write drops every cached page, so an author sees their own post at once.
- Entries are capped, so a caller sending a distinct search per request can't grow the map without bound; the rate limiter bounds how much such a caller can provoke.

## Consequences

Across instances the TTL is the real staleness bound. Like the rate limiter ([12](0012-assistant-entitlement-is-a-subscription.md)), revisit it if the service scales past one instance.
