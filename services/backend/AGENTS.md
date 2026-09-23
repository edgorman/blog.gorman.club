# services/backend

The Go backend on Cloud Run. See also `README.md` here, `packages/protos/AGENTS.md` for the wire contract, and `infrastructure/AGENTS.md` for the environments it runs in.

## AI Writing Assistant

The backend calls Gemini models on the **Gemini Enterprise Agent Platform** — the product formerly called Vertex AI, whose API is still served at `aiplatform.googleapis.com` (its discovery document is now titled "Agent Platform API"). It authenticates as the Cloud Run runtime service account (`roles/aiplatform.user`) over Application Default Credentials rather than with an API key, which is the same reasoning that put CI on Workload Identity Federation: there is no long-lived credential to store in Secret Manager, rotate, or leak.

The platform is chosen over the **Gemini API** (`generativelanguage.googleapis.com`) for exactly that reason. The Gemini API's `generateContent` declares no OAuth scope in its discovery document, so it accepts an API key and nothing else; a token-authorized request to it is refused with a `403`. Only this platform's `generateContent` declares the `cloud-platform` scope, and so accepts the credential the deployment already has. Reaching for the Gemini API instead would mean introducing the one long-lived secret this architecture is built to avoid.

The model id and its location are Terraform variables (`assistant_model`, `assistant_location`) rather than constants, since model availability is regional and model ids change faster than this service is redeployed.

Access is an entitlement, and it is expressed in the same access model as every other rule (see Access Control below): a whitelist whose membership is worked out per request rather than stored on a document. An account is entitled while its subscription has not expired (`subscribedUntil` on its profile) and no other way - there is no per-environment allowlist to be on, so granting access is writing that field and revoking it is clearing it, neither of which needs a deploy. It is set by hand until a checkout writes it. The subscription is keyed on the account rather than on an address: the profile is loaded by the uid in the verified ID token, so nothing a request asserts about itself is consulted, and a username - freely chosen and, once released, claimable by anybody - is never what access follows.

The entitlement says who may spend, not how much, so volume is bounded separately: assistant turns are rate limited per account by the backend, on a much tighter budget than any other route, because a turn is the only request that calls a paid model. The buckets are held in the serving process, which makes them per-instance budgets and is the one thing to revisit if the service ever scales past a single instance (see `services/backend/README.md`).

## Access Control

Who may do what is one model rather than a rule per feature (`services/backend/internal/entity/access.go`). Every **resource** - a post, a profile, a comment, a reaction, and the assistant as a gated feature - declares an **access** mode for each **action** on it (`read`, `create`, `update`, `delete`), and there are exactly three modes: public (everybody, signed in or not), private (the owner alone), and whitelist (the owner plus whoever was named beside them). A single policy table holds every pair; one it does not declare is refused, so a feature added without a line in it is closed rather than open.

A post is the one resource that chooses its own read audience: its `visibility` and `allowedUserIds` *are* the three modes, read back as one. The assistant is the one whitelist that is not a field on a document - its membership is the subscription above. There are deliberately no roles: every rule here is decided by who owns a thing, who was named on it, or what an account has paid for, all of which a role would sit between rather than answer. See `services/backend/README.md` for how a permission is asked and why a refused read is a `404` rather than a `403`.

## Reader Engagement

Posts carry comments and reactions, both stored in Firestore beneath the post
they belong to and both governed by the post's own read rules rather than rules
of their own. Reactions are a fixed set of five emoji (`entity.AllowedEmojis`),
not custom or combined ones, and they are addressed (`PUT`/`DELETE`) rather
than toggled so a retried click is harmless. See `services/backend/README.md`
for the ownership and moderation rules.

## Finding Posts

The feed is reverse-chronological, so `GET /blogs` carries two filters beside
the `ownerId` a profile feed uses: `tag` narrows to one topic and `q` to a
case-insensitive substring of a post's title or body. Both narrow the same feed
and neither can widen it - each is applied on top of the read rules above, so a
search can never surface a post the caller could not already have scrolled to,
and a tag says nothing about who may read a post.

Tags are normalized on the way in (lowercase, one hyphen between words), so the
form a post is stored, filtered, and linked under is decided by the server
rather than by however an author typed it. A tag becomes a Firestore
`array-contains` filter with its own composite indexes, and replaces rather than
joins the readability OR the general feed uses - a query may hold only one
`array-contains` clause, and a tag is the more selective of the two, so
readability falls back to the same in-Go filtering a profile feed already
relies on.

Search is deliberately a substring scan applied as the feed is walked, not a
search index: there is no Firestore predicate for it, and at this scale the
alternative is a service to run, pay for, and keep in step. It is bounded and
paged, and is the first thing to revisit if the collection outgrows it.

## Caching

`GET /blogs` is the highest-traffic and most repeated read in the service - the
landing feed is the same public data for every signed-out visitor - so the
anonymous listing is cached in the serving process for a short TTL
(`services/backend/internal/repository/cache`). It is a decorator over the blog
repository rather than something a handler does, wired in `cmd/backend`: the
service cannot tell a cached page from a fetched one, and the cache is removed by
deleting a line.

Only the anonymous caller's pages are cached, and that is what makes it safe
rather than something to be careful with. A page is whatever the read rules above
admit for one uid, so two callers may share an answer only if they share a uid -
and the empty uid, which is not an account at all, is granted public posts and
nothing else. A signed-in caller's page carries their own private and whitelisted
posts, so it is neither stored nor served here; there is no per-uid keying to get
wrong because there are no per-uid entries. Every filter is part of the key, so a
tag or a search is its own entry rather than a variation on the feed.

A write drops every cached page, so an author sees their own post in the feed at
once; across instances the TTL is the real bound, since the cache - like the rate
limiter's buckets - is held per-instance and is the same thing to revisit if the
service scales past one. Entries are capped, because a caller sending a distinct
search term per request would otherwise grow the map indefinitely; the read volume
such a caller can provoke is bounded by the rate limiter rather than by the cache.

## Container image under Pants

The backend's container image is modeled as a `docker_image` target (`services/backend:image`) over the existing multi-stage Dockerfile unchanged, not - as #117 first tried - a `go_binary` dependency with the Dockerfile reduced to its runtime stage. That would have meant the packaged binary only exists inside a Pants-assembled build context, breaking the plain `docker build services/backend` that `backend-deploy/action.yml` still runs on every merge; #117 leaves that path alone. The `COPY . .` this keeps means Pants' Dockerfile dependency inference - which only resolves literal `COPY` sources, not `.` - can't find the target's dependencies on its own, so they're declared by hand as `services/backend/cmd/backend`, whose ordinary Go import inference already reaches every `internal/` package it imports; that's the edge `pants dependents` needs to reach the image from an `internal/` change. `extra_build_args=["COMMIT_SHA"]` mirrors the `--build-arg COMMIT_SHA` the deploy action already passes, reading it from whatever CI sets before invoking `pants package` and falling back to the Dockerfile's own `ARG COMMIT_SHA=unknown` locally. `[docker.registries]` in `pants.toml` registers the real `stag`/`prod` Artifact Registry paths so a target can reference them by `@stag`/`@prod`, but both are `skip_push` and neither `default` - nothing publishes a Pants-built image yet. `pull-request.yaml`'s `test` job does now run `pants ... test package` (#156), so this target is built - not published - on every PR that reaches it, closing the gap #120 opened when it retired the advisory `pants package services/backend::` step along with the job that ran it.
