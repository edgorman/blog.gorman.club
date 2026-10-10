# 14. Filters and search only narrow the feed

Status: Accepted

## Context

`GET /blogs` is a reverse-chronological feed with three filters: `author` (a username), `tag` and `q` (search by meaning). `GET /blogs/{slug}/related` ranks other posts by meaning.

## Decision

- Every filter is applied on top of the read rules, so none can surface a post the caller couldn't already scroll to. A tag says nothing about who may read a post.
- Tags are normalized on the way in (lowercase, one hyphen between words), so the server decides the form a tag is stored, filtered and linked under.
- A tag is a Firestore `array-contains` filter with its own composite indexes. A query may hold only one `array-contains`, and a tag is more selective than the readability OR, so with a tag readability falls back to the in-Go filtering a profile feed already uses.
- Search: `internal/repository/search` decorates the datastore repository. It embeds `q` (the worker's `EMBEDDING_MODEL` and dimension, `RETRIEVAL_QUERY` against posts' `RETRIEVAL_DOCUMENT`), runs `FindNearest` over `embeddings/`, then loads each candidate and keeps it only if `CanBeReadBy` the caller and it passes `author` and `tag`. The index only ranks. Relevance has no `createdAt` cursor, so a search is one page (`hasMore: false`).
- If the query can't be embedded or the index can't be read, the substring scan answers instead of an error. With no embedding model configured the decorator isn't installed.
- A search is a paid model call even for anonymous callers, so `q` has its own rate-limit bucket (`searchesPerClient`, by uid or IP), and the anonymous cache sits in front of the decorator.
- Related posts run `FindNearest` from the post's own vector, ask the post's read rule first and filter every candidate by `CanBeReadBy`. A post with no embedding yet has no related posts rather than an error.

## Consequences

A paraphrase finds a post that shares no word with it, and search can never widen what a caller sees.
