# services/backend

The Go backend on Cloud Run. See also `README.md` here, `packages/protos/AGENTS.md` for the wire contract, and `infrastructure/AGENTS.md` for the environments it runs in.

## AI Writing Assistant

The backend calls Gemini models on the **Gemini Enterprise Agent Platform** — the product formerly called Vertex AI, whose API is still served at `aiplatform.googleapis.com` (its discovery document is now titled "Agent Platform API"). It authenticates as the Cloud Run runtime service account (`roles/aiplatform.user`) over Application Default Credentials rather than with an API key, which is the same reasoning that put CI on Workload Identity Federation: there is no long-lived credential to store in Secret Manager, rotate, or leak.

The platform is chosen over the **Gemini API** (`generativelanguage.googleapis.com`) for exactly that reason. The Gemini API's `generateContent` declares no OAuth scope in its discovery document, so it accepts an API key and nothing else; a token-authorized request to it is refused with a `403`. Only this platform's `generateContent` declares the `cloud-platform` scope, and so accepts the credential the deployment already has. Reaching for the Gemini API instead would mean introducing the one long-lived secret this architecture is built to avoid.

The model id and its location are Terraform variables (`assistant_model`, `assistant_location`) rather than constants, since model availability is regional and model ids change faster than this service is redeployed.

Access is an entitlement, and it is expressed in the same access model as every other rule (see Access Control below): a whitelist whose membership is worked out per request rather than stored on a document. An account is entitled while its subscription has not expired (`subscribedUntil` on its profile) and no other way - there is no per-environment allowlist to be on, so granting access is writing that field and revoking it is clearing it, neither of which needs a deploy. The Stripe webhook is what writes it (see Billing below). The subscription is keyed on the account rather than on an address: the profile is loaded by the uid in the verified ID token, so nothing a request asserts about itself is consulted, and a username - freely chosen and, once released, claimable by anybody - is never what access follows.

The entitlement says who may spend, not how much, so volume is bounded separately: assistant turns are rate limited per account by the backend, on a much tighter budget than any other route, because a turn is the only request that calls a paid model. The buckets are held in the serving process, which makes them per-instance budgets and is the one thing to revisit if the service ever scales past a single instance (see `services/backend/README.md`).

## Billing

The assistant is sold as one tier through Stripe. What is bought is time rather than a plan: a single recurring Price, and `subscribedUntil` is the subscription's current period end. The only other billing fact stored is the Stripe customer id (`stripeCustomerId`), kept so a later checkout reuses the customer and the portal can be opened for it. The adapter (`internal/repository/stripe`) speaks REST over `net/http` with no Stripe SDK, for the same reason `internal/repository/gemini` hand-writes its protocol.

- `POST /billing/checkout` and `POST /billing/portal` answer `{ url }` (`blogv1.BillingSessionResponse`) to a Stripe-hosted page, for the caller's own account only. Checkout records the uid on the subscription's metadata, which is how the webhook finds the account again. A live subscriber is refused a second checkout (`409`); cancelling happens in the portal.
- `POST /billing/webhook` is the **only** thing that grants access and the only route with no credential. It verifies the `Stripe-Signature` HMAC over the raw body (constant-time compare, five-minute replay window) before parsing anything, acts on `customer.subscription.created/updated/deleted`, and **re-fetches the subscription** instead of trusting the event, so an out-of-order or retried event writes the current state rather than a stale one; fetch and write run under one per-process lock so of two racing deliveries the later fetch writes last. Active, trialing or past_due (a failed renewal Stripe is still retrying) sets `subscribedUntil` to the period end; any other status clears it, unless the stored date is later than that subscription's period end (a newer subscription paid for it). Handled or ignored events answer 2xx, a bad signature 400, and a failed fetch or write 500 so Stripe retries.
- A profile write can never grant access: `UserRepository.Put` keeps the stored `subscribedUntil` and customer id whatever it is handed, and only `SetSubscription` writes them.
- `DELETE /users/me` answers `409` while a subscription is live, since the billing would outlive the account it pays for.
- Returning from Checkout proves nothing. The success URL (`/user/{username}?checkout=success`) only makes the frontend say the subscription will appear shortly and re-read `/users/me`.

Billing is off unless `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` and `STRIPE_PRICE_ID` are all set (and `CORS_ALLOWED_ORIGIN`, which the return URLs are built from). Off means the three routes are not registered at all (`404`) and `/users/me` reports `billingEnabled: false`, so the frontend offers no button. Terraform (`infrastructure/env/billing.tf`) creates the two secrets and the runtime's access to them per environment but never their values, and only mounts them on Cloud Run once `stripe_price_id` is set. To go live in an environment, in this order:

1. Merge, so the apply creates the `stripe-secret-key` and `stripe-webhook-secret` secrets (empty).
2. In the Stripe dashboard (test mode for staging), create the Product and its recurring Price, and register a webhook endpoint at `<backend URL>/billing/webhook` for `customer.subscription.created`, `customer.subscription.updated` and `customer.subscription.deleted`.
3. Add a version to each secret by hand (`gcloud secrets versions add stripe-secret-key --data-file=-`, and the endpoint's signing secret to `stripe-webhook-secret`), never through tfvars.
4. Set `stripe_price_id` in that environment's `infrastructure/config/<env>/terraform.tfvars` and merge, so the apply mounts the secrets. A revision whose secret has no version fails to start, which is why this comes last.
5. Enable the Customer Portal in the Stripe dashboard with cancellation allowed.

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
search of posts by meaning (below). Both narrow the same feed
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

Search is by meaning. `internal/repository/search` decorates the datastore
repository: when `q` is set it embeds the query synchronously (the same
`EMBEDDING_MODEL` and dimension the worker embeds posts with, as
`RETRIEVAL_QUERY` against the posts' `RETRIEVAL_DOCUMENT`), runs `FindNearest`
over `embeddings/`, loads each candidate and keeps it only if `CanBeReadBy` the
caller and it passes `ownerId`/`tag`, and returns them most relevant first. So
a paraphrase ("deploying containers") finds a post that shares no word with it
("Shipping to Cloud Run"), and the index still only ranks. Relevance has no
`createdAt` cursor, so a search is one page: the top `limit` with
`hasMore: false`.

The substring scan stays as the fallback. When the query can't be embedded or
the index can't be read, the search is answered by it instead of failing, and
with no embedding model configured the decorator isn't installed at all. The
scan is applied as the feed is walked, bounded and paged, newest first.

A search is one paid model call, including for anonymous callers, so `q`
requests have their own rate-limit bucket (`searchesPerClient` in
`internal/service/ratelimit.go`, by uid or by IP), and the anonymous cache sits
in front of the decorator, so a repeated anonymous search costs no model call.

Related posts (`GET /blogs/{slug}/related`) rank by meaning rather than by
time: the worker embeds every post's title and body into `embeddings/{slug}`,
and the route runs a Firestore `FindNearest` (cosine) over that collection from
the post's own vector. The index only ranks - every candidate is loaded and kept
only if `CanBeReadBy` the caller, and the post's own read rule is asked first -
so it inherits the same guarantee as `q` and `tag`. A post with no embedding yet
has no related posts rather than an error.

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

## Container image

The backend's container image is a `services/backend:image` moon task (`services/backend/moon.yml`) over the existing multi-stage Dockerfile unchanged: `docker build --build-arg COMMIT_SHA="${COMMIT_SHA:-unknown}" -t backend:${COMMIT_SHA:-local} .`, matching the `--build-arg COMMIT_SHA` `backend-deploy/action.yml` already passes on every merge and falling back to the Dockerfile's own `ARG COMMIT_SHA=unknown` locally. Its `inputs` are `@group(sources)` (every `.go`/`go.mod`/`go.sum` file) plus `Dockerfile` and `.dockerignore` directly - a plain glob, not a resolved dependency edge, so an `internal/` change marks the task affected the same way any other source change would, with nothing hand-declared to keep in sync. `options.cache: false` reflects that the task's real output is a local Docker image layer moon has no way to restore from its own cache, not a file it could hydrate; `runInCI: true` keeps it from being treated as a local-only/server task despite that. `pull-request.yaml`'s `test` job runs `moon ci :test services/backend:image`, so this task is built - not published - on every PR that reaches it, the same as any other required check; nothing here pushes to a registry, that's `backend-deploy/action.yml`'s own job on merge to `main` (see "Staging Deployments" in `.github/AGENTS.md`).

## Worker

`cmd/worker` is a second binary in this module, for work that runs *after* a Firestore write rather than in the request that made it. It lives here rather than in its own module because it needs `internal/entity` and `internal/repository/firestore`, which Go's `internal/` rule keeps inside this module; the handlers are in `internal/worker`.

- **Events.** Eventarc triggers (`infrastructure/env/worker.tf`) deliver `google.cloud.firestore.document.v1.written` on `blogs/{slug}` to `POST /events/blog` and `...document.v1.created` on `blogs/{slug}/comments/{id}` to `POST /events/comment`, as binary-mode CloudEvents. Their body is a protobuf `DocumentEventData` (Firestore events offer no JSON encoding, so the triggers set `event_data_content_type = "application/protobuf"` explicitly: `application/json` and an unset value both make trigger creation fail), and the worker doesn't read it: `Ce-Type` and `Ce-Document` name the event and document, and a handler that needs the contents reads the document from Firestore.
- **Status codes decide retries.** Eventarc redelivers anything non-2xx. A handler returns an error only when trying again could succeed (answered 500); a handled or deliberately ignored event is answered 204. Handlers must be idempotent and a no-op when nothing they care about changed, since the worker's own writes can fire its triggers again.
- **Not public.** `ingress = INGRESS_TRAFFIC_INTERNAL_ONLY` and no `allUsers` invoker: only the `worker-trigger` service account holds `run.invoker`. It runs as `worker-runtime` (`datastore.user`, `aiplatform.user`).
- **Logs** are one JSON line per event on stdout, keyed `message`/`severity` so Cloud Logging parses them. The `worker-<env> is failing events` alert fires on a run of 500s.
- **Embeddings.** `POST /events/blog` re-reads the post and writes `embeddings/{slug}` (vector, `contentHash`, model, `ownerId`) with the Agent Platform's `:predict` on `EMBEDDING_MODEL` (Terraform `embedding_model`, `embedding_location`, `embedding_dimension`; the last also sizes the vector index in `firestore.tf`). A write whose title and body hash, and model, match what is stored makes no model call; a missing or soft-deleted post has its embedding deleted. They live in their own collection so the worker never writes `blogs/`, which would fire its own trigger.
- **Backfill.** On every start, before it listens, the worker syncs every post (bounded to two minutes, failures logged rather than fatal). A deploy always starts an instance, so posts written before embeddings existed are covered by the first deploy, with no hand-run step; posts already in step cost two reads each and no model call.
- **Comment moderation** (`internal/worker/moderation.go`). Comments are published first and hidden afterwards if flagged, so a model outage leaves comments unscreened rather than unposted. On each new comment the worker reads it back, skips it if it already carries `moderation`, classifies its body with `gemini.Moderator` (structured JSON output at temperature 0; the comment is sent JSON-encoded inside a `<comment>` data block, never in the instructions) and writes `moderation: {status, category, model, at}`. Anything but a well-formed verdict is a 500 and is retried, never read as approval. `GET /blogs/{slug}/comments` drops flagged comments for everyone but their author (who sees them unmarked) and the post's owner (who alone is sent `moderation`), and `PUT /blogs/{slug}/comments/{id}/moderation` lets the owner approve one. The model is `moderation_model`/`moderation_location` in Terraform, like the assistant's.
- **Build and deploy.** The Dockerfile's `CMD` build arg picks the binary (`backend` by default). The `worker` moon project (`cmd/worker/moon.yml`) has only `worker:image`; formatting, vet and tests stay with `services/backend`'s `./...` tasks. On merge, `push-commit.yaml`'s `services-worker` publishes `backend/worker:<sha>` to both registries through `backend-deploy` and deploys `worker-stag`, and `promote-release.yaml` deploys that same image to `worker-prod`. It shares the `backend` repository's per-package keep-count retention.
