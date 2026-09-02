# backend

Go backend service for blog.gorman.club, packaged as a Docker container and
deployed to GCP Cloud Run (see the repository root `CLAUDE.md` for the full
deployment architecture).

It implements the Debug Endpoint Contract: `/health` and `/debug` both
return system status, a timestamp, the deployment environment, and the git
commit SHA the running image was built from, as JSON:

```json
{
  "status": "ok",
  "timestamp": "2026-08-22T12:00:00Z",
  "environment": "stag",
  "commit": "abc1234"
}
```

## Layout

```
cmd/backend/          Entrypoint: reads config, builds adapters, hands them to the service
internal/
  entity/             Domain types and their rules - no I/O, no HTTP, no persistence tags
  repository/         Interfaces for everything external, plus ErrNotFound
    firestore/        Firestore implementations of the Blog, User, Chat, Comment and Reaction repositories
    google/           Google Identity Services implementation of TokenVerifier
    gemini/           Gemini Enterprise Agent Platform implementation of Assistant
  service/            HTTP server: routes, middleware, and logic spanning more than one entity
```

Dependencies point inward: `service` and `repository` both know `entity`, `entity` knows nothing
else, and only `cmd/backend` knows which concrete adapters exist. Swapping Firestore for another
store means adding a folder under `repository/` and changing one line in `cmd/backend`.

Validation lives on the entities as setters (`SetUsername`, `SetVisibility`, ...) that trim,
check, and only then apply, so a rejected value is never half-written. The HTTP layer decodes into
a request struct holding only the client-settable fields and applies it through those setters -
which is why `ownerId` and `createdAt` cannot be spoofed: they are not part of the request shape at
all.

`Validate` runs those same setters against a copy and additionally requires the server-set identity
(`Blog.OwnerID`, `User.ID`). Repositories call it before every write and return without touching
the datastore if it fails, so the rules hold for any entity, however it was assembled.

Entities carry `json` tags only. Firestore's field names live on separate document structs in
`repository/firestore`, which also keeps the document body free of the ID that Firestore already
stores as the document key.

## Access control

Every gate in this service is one question asked of one model
(`internal/entity/access.go`), rather than a rule invented where it happened to
be needed - which is what the assistant's email check used to be, bolted onto
the config beside a post's own visibility rules.

The model has three parts and no more:

- A **resource** is a kind of thing the service holds (`user`, `blog`,
  `comment`, `reaction`) or a feature it gates (`assistant`).
- An **action** is one thing that can be done to it: `read`, `create`,
  `update`, `delete` - the same four verbs the routes are registered under.
- An **access** mode is how wide the audience for that pair is. There are three:
  **public** (everybody, signed in or not), **private** (the owner alone), and
  **whitelist** (the owner plus whoever was named beside them).

The `policy` table declares a mode for every (resource, action) pair that
exists, and is the one place to read to know what this service allows. A pair it
does not name carries no mode at all and is refused, so a feature added without
a line in the table is closed rather than open.

Asking is two steps and always the same two. An entity turns the declared mode
into a `Permission` narrowed to one particular thing - who owns it, who was
named on it - and `Permission.Allows(uid)` answers. The zero uid is the
anonymous caller and holds nothing but public access however the rest of the
permission is filled in: an ownerless thing must never make a signed-out request
its owner, which is the one way this could have failed open.

There are deliberately **no roles**. A role would be a name for a set of
permissions, and every permission here is decided by who owns the thing or who
was named on it - which a role sits between rather than answers. The one gate
that is not about ownership, the assistant, is a whitelist whose membership is
bought (below), and that is a lookup rather than a role too.

Several actions are declared private for a caller who could only ever be acting
on their own: a profile is addressed as `/users/me`, a reaction is keyed by the
reader who left it, and a post is created owned by whoever asked. Those are
enforced by the address rather than by a check - the forbidden case is
unreachable, not refused - and they are declared anyway, because a table that
only listed the checks that happen would not say what the rules are.

A post is the one resource whose read audience is not fixed by the table: it is
whatever the post itself says. `Blog.Permission(ActionRead)` reads a post's
`visibility` and `allowedUserIds` back as a mode - public is everybody, private
is the owner, and naming readers is what widens private into a whitelist - so
the three modes an author can choose between are exactly the three the model
has. A stored post carrying a visibility this build does not recognise reads as
private, which is the way round to get an unknown value wrong.

Permissions compose by being asked in order rather than by growing a fourth
mode. A comment's thread is readable by whoever may read the post above it: the
post's read permission is asked first and a caller who fails it gets the post's
own `404`, then the comment's own permission decides the rest.

## Users and blogs

Data lives in Firestore (`infrastructure/env/firestore.tf`). This service is
the only client of that database: it authenticates with the Admin SDK, no
Firestore security rules are deployed, and the browser never talks to
Firestore directly. Access rules therefore live in Go and nowhere else:

- **Reads** — `Blog.CanBeReadBy` - the post's own read permission, asked by its
  most common name - is the single definition of who may see a post (public
  posts for anyone, signed in or not; private ones for the owner or a uid in
  `allowedUserIds`). `GET /blogs` applies it as a Firestore query so private
  posts are never fetched; `GET /blogs/{slug}` applies it to the loaded
  document. Both routes run `optionalAuth`: a signed-in caller's own private
  and whitelisted posts are included, but no credential is required.
- **Writes** — `requireBlogPermission` restricts updates and deletes to the
  post's owner, and `createdAt`/`updatedAt` come from the server rather than a
  spoofable client clock.

A post's address is a slug taken from its own title - so unlike the random key a
Firestore-assigned id would have been, it is guessable. `blogFromPath` and
`requireReadableBlog` (`internal/service/blog.go`) answer every way of missing - a
malformed slug, a slug nothing holds, and a post the caller is not allowed to read -
with the same `404`. A `403` would tell a prober a private post exists at a path it
merely guessed right; folding that case into "not found" is what keeps the guess from
confirming anything. `requireBlogPermission` layers the action's own permission
on top of readability: a caller who cannot read a post gets the masking `404`
before ownership is even considered, and only a post they can see yields a `403`
for not owning it - which discloses nothing they could not already see. Asking
it for `read` therefore checks the same thing twice and answers `404` either
way, which is all `requireReadableBlog` is.

A post has no id of its own. It is addressed by a **slug** taken from its title
(`Hello, world!` → `/blogs/hello-world`), and keyed in Firestore by that slug
alone, so uniqueness is a property of the document key rather than something
checked: `Create` (unlike `Set`) refuses to overwrite an existing document.
Slugs are therefore unique *across every author*, which is what lets the author
be left out of the address entirely - the second post under a title falls back
to a suffixed slug (`hello-world-k3m9x`) whoever wrote it, drawn rather than
counted up because only the write can decide whether one is free. A slug is
assigned once and never revised, so retitling a post leaves every link to it
working.

A handful of slugs are reserved (`entity.reservedBlogSlugs`) because the
frontend routes them elsewhere: `new` is its editor at `/post/new`, which
outranks the `:slug` wildcard beside it, so a post there would be unreachable.
A title that slugs to one takes the suffixed form straight away, and `SetSlug`
refuses a reserved slug outright.

Profiles are keyed by the owner's Google account ID, so there is no
server-assigned ID to hand out and a profile is written with `PUT` rather than
`POST`. That key is never a URL, though: a profile is addressed by its
**username**, which is assigned at sign-up from two descriptive words and an
animal (`calm-smiling-kestrel`) and may be changed later. Uniqueness is enforced
by a `usernames/{lowercased}` collection whose document key *is* the constraint,
claimed in the same transaction as the profile write - so two callers picking the
same name cannot both succeed.

A username is the whole of a public identity. There is deliberately no display
name: one that could be set freely would let anyone present themselves under a
name somebody else holds, which is exactly what making the unique handle the
visible one prevents.

Writes are addressed as `/users/me` and resolve the owner from the verified
credential, so there is no id in the path to point at somebody else's profile -
the forbidden case is unreachable rather than merely checked. `GET /users/me`
exists because a client holds a credential, not a username, and is how it learns
the name it was given.

Because a post records its owner by uid, and a uid resolves to nothing over
HTTP, blog responses carry an `authorUsername` resolved at read time - the only
handle a client holds for the profile behind a post, and so what it links the
author by. Publishing therefore assigns a profile to an author who somehow
reached `POST /blogs` without one, since a post by an unnamed author would be
attributed to nobody. It is empty only for a post written before that rule,
whose owner still holds no profile.

Every route below except `GET /blogs`, `GET /blogs/{slug}`,
`GET /blogs/{slug}/comments`, `GET /blogs/{slug}/reactions` and
`GET /users/{username}` requires a Google Sign-In credential — those five run
`optionalAuth` instead of `requireAuth`, so an anonymous request is answered as
far as public posts allow rather than rejected. A profile has nothing
caller-specific to hide, so it is readable either way. A signed-in caller sends the ID token Google issued, plus a header
naming the provider that issued it:

```
Authorization: Bearer <google-id-token>
Authorization-Provider: google
```

`requireAuth` and `optionalAuth` (`internal/service/auth.go`) share the same
verification: signature, expiry, audience (`GOOGLE_CLIENT_ID`) and issuer are
checked against Google's public keys, and the resulting identity — id, email,
name, all from the signed payload — goes into the request context. They differ
only in how they treat a request with no `Authorization` header at all:
`requireAuth` rejects it, `optionalAuth` lets it through as the zero identity,
which `Blog.CanBeReadBy` already treats as seeing only public posts. Either
way, nothing the client sends about itself is trusted, and a credential that
*is* present but invalid is rejected the same way by both.

Failures are distinguished so the cause is obvious from the status alone:

| Status | Meaning                                                                 |
| ------ | ------------------------------------------------------------------------ |
| `401`  | A route that requires one got no credential, or one that failed verification. |
| `400`  | Headers missing or malformed — the caller didn't ask properly.            |
| `501`  | A provider this service doesn't implement.                                |
| `500`  | `GOOGLE_CLIENT_ID` is unset, so the deployment can't verify anything.     |

Adding another provider means extending `authProvider` and the switch in
`requireAuth`, plus a new implementation of `repository.TokenVerifier` alongside
`repository/google` - not restructuring the flow.

| Method | Path          | Description                                                           |
| ------ | ------------- | --------------------------------------------------------------------- |
| GET    | `/blogs`      | List the blogs the caller may read, newest first. No credential required. |
| GET    | `/blogs/{slug}` | Fetch a single blog. No credential required for a public one; a private one the caller may not read is a `404`, the same as a missing one. |
| POST   | `/blogs`      | Create a blog. `ownerId` is always the caller, and the slug comes from the title, regardless of the body. |
| PUT    | `/blogs/{slug}` | Replace a blog's fields. A post the caller may not read is a `404`; one they may read but do not own is a `403`. The slug does not move, even when the title changes. |
| DELETE | `/blogs/{slug}` | Delete a blog. Same `404`/`403` split as `PUT`.            |
| GET    | `/users/{username}` | Fetch a profile by username. No credential required. A 404 doubles as the availability check. |
| GET    | `/users/me`   | Fetch your own profile, including the username you were assigned.      |
| PUT    | `/users/me`   | Create or replace your own profile. An omitted `username` keeps the one you hold; a taken one is a `409`. |
| DELETE | `/users/me`   | Delete your own profile, releasing its username.                       |
| GET    | `/blogs/{slug}/chat` | Fetch the assistant conversation about a post. A post nobody has discussed is an empty conversation, not a `404`. |
| POST   | `/blogs/{slug}/chat` | Send the assistant a message. Applies whatever it edits to the post and answers with the exchange plus the post as it now stands. |
| DELETE | `/blogs/{slug}/chat` | Throw the conversation away. The post, edits included, is untouched. |
| GET    | `/blogs/{slug}/comments` | List the comments on a post, oldest first. No credential required for a public post; a post the caller may not read is the same `404` the post itself gives. |
| POST   | `/blogs/{slug}/comments` | Comment on a post. `authorId` is always the caller, whatever the body says. |
| DELETE | `/blogs/{slug}/comments/{id}` | Delete a comment. Its author or the post's owner may; anybody else who can read the post gets a `403`, and an id naming nothing is a `404`. |
| GET    | `/blogs/{slug}/reactions` | Every reaction on the post and on its comments, as counts. No credential required for a public post. |
| PUT    | `/blogs/{slug}/reactions/{emoji}` | React to the post. Idempotent, and answers with the post's counts as they now stand. |
| DELETE | `/blogs/{slug}/reactions/{emoji}` | Take your reaction back. Idempotent in the same way. |
| PUT    | `/blogs/{slug}/comments/{id}/reactions/{emoji}` | React to one comment. A comment that does not exist is a `404`. |
| DELETE | `/blogs/{slug}/comments/{id}/reactions/{emoji}` | Take that reaction back. |

### Response shape

Successful responses return the resource itself (`201 Created` the first time
a profile is `PUT`, `200 OK` thereafter; `204 No Content` for `DELETE`). Every non-2xx response is JSON of the same shape, so a client can
parse success and failure the same way:

```json
{ "error": "blog not found" }
```

## Comments

The `/blogs/{slug}/comments` routes are the readers' half of a post, and hang
off it for the same reason the assistant chat does: a comment has no identity
apart from the post it replies to, and no route could name one that a
`/blogs/{slug}` route would not have resolved first. They are stored that way
too — `blogs/{slug}/comments/{id}` — which keeps a thread a single-collection
query that needs no composite index, and gives a comment an id that means
nothing outside its post.

- **Who may read one.** Exactly whoever may read the post: `ListComments` goes
  through the same `requireReadableBlog` every other read does, so a private
  post's thread is as private as the post, and a caller who cannot see one gets
  the post's own `404` rather than an empty thread — which would itself admit
  the post exists.
- **Who may write one.** Any signed-in caller who may read the post, its author
  included. Reading a thread never needs a credential; writing to one always
  does, since a comment is signed by whoever left it, and an anonymous comment
  would be attributable to nobody. A commenter with no profile is given one, for
  the same reason publishing gives an author one: they are shown by username.
- **Who may delete one.** `entity.Comment.Permission(ActionDelete, post)` is the
  single definition, and it is a whitelist of exactly one name beside the
  comment's author: the owner of the post it sits under. That second name is
  what makes this moderation rather than only retraction — an author is
  answerable for what appears beneath their post. There is deliberately no way
  to *edit* a comment at all, by anyone, so moderating cannot become putting
  words in somebody's mouth: a comment is written and removed, never rewritten.

A comment is erased on delete rather than soft-deleted like a post. A post's
absence would be a hole in the record; a comment being taken down has to
actually remove what was said.

The frontend renders a comment body as text rather than as markdown, unlike the
post above it — the safe rendering of a stranger's input is the one with no
syntax in it.

## Reactions

Readers react to a post and to the comments on it with an emoji, which is the
lightest thing a reader can say. The rules follow the post, as comments' do: a
reader who may read a post may see and add reactions to it and to its comments,
and one who may not gets the post's own `404`.

- **A fixed set of five, not any emoji.** `entity.AllowedEmojis` (👍 👎 ❤️ 😄
  🎉) is the whole of what a reaction may be, and `entity.ValidEmoji` is exact
  membership in it — not a shape check, so a composed variant of one of the
  five (a skin tone, say) does not match the plain glyph it modifies. There is
  no custom-emoji upload and no combining runes of your own. Widening the set
  is a change to that one array on the backend and the matching array in the
  frontend's `ReactionBar`, and nothing else.
- **Addressed, not toggled.** `PUT` puts a reaction there and `DELETE` takes it
  back, both idempotent, so a retried click or a stale page lands where it was
  aiming rather than undoing itself. The client decides which to send from what
  it last read; the server never flips. Both answer with the target's counts as
  they now stand, since a bar is a shared number that one client's own click
  cannot predict.
- **Counted, not named.** A reaction is reported as an emoji, a count, and
  whether *you* are in it. Who else reacted is deliberately not disclosed:
  naming them would turn a one-click gesture into a public record of who liked
  what, which is a heavier thing than the button suggests.

One reader's reactions to one thing are stored as a single document keyed by
that pair (`blogs/{slug}/reactions/{target}-{uid}`), so "this reader, this
target" is unique by construction — the same argument that keys a post by its
slug — and two readers reacting at once write different documents and never
contend. A comment's reactions live beside the post's rather than beneath the
comment, which is what makes a page one query instead of one per comment.
A reader is bounded by `entity.AllowedEmojis` itself rather than by a separate
limit — once all five are chosen there is nothing left to add, and none may
repeat. How many readers may react is unbounded.

Deleting a comment deletes its reactions too, so a moderated comment cannot
survive as a row of numbers. That cleanup is best-effort and logged rather than
returned: the caller asked for the comment to be gone, and it is.

## Rate limiting

Every request is metered by a token bucket before it reaches a handler
(`internal/service/ratelimit.go`). Three budgets apply, outermost first:

| Budget       | Keyed on                    | Allowance                      | Covers                                            |
| ------------ | --------------------------- | ------------------------------ | ------------------------------------------------- |
| Per client   | client address              | 60 at once, +1 per second      | every route, signed in or not                     |
| Per account  | the uid in a verified token | 20 at once, +1 per 3 seconds   | every route behind `requireAuth` - so every write |
| Assistant    | the uid in a verified token | 5 at once, +1 per 30 seconds   | `POST /blogs/{slug}/chat`                         |

They are layered rather than alternatives, because each answers a question the
others cannot. Only the address is known before a credential has been checked,
so it is the only thing that can bound an anonymous flood - including one aimed
at the token verifier itself, which is not free. Only the account survives a
caller changing address, so it is the only thing that can bound somebody who
signed in. And the assistant needs a budget far below either, because a chat
turn is the one request here that calls a paid model and can hold a connection
open for two minutes: an allowance that is generous for editing a post is
ruinous for it.

The client address is the **rightmost** `X-Forwarded-For` entry, not the
leftmost one usually wanted for logging. Anything to the left of it is whatever
the client chose to send - Cloud Run appends the address it saw rather than
replacing the header - so metering the leftmost entry would let a caller dodge
the limit with a random header per request. Putting a proxy in front of the
service would shift the real client one entry to the left and collapse everyone
into one bucket: over-limiting rather than under-limiting, which is the way
round to get this wrong.

A refused request is a `429` carrying the usual JSON error body and a
`Retry-After` header. The wait is repeated in the message because a browser
cannot read a header this API does not expose to it, and the whole of what an
author needs to know is when to try again.

Buckets live in the memory of one process, so these are per-*instance*
budgets: a service running on N instances admits N times the numbers above,
and a caller whose requests land on different instances is metered separately
on each. That is fine while this deployment runs as a single instance, and it
is the thing to revisit before scaling out - the counters would have to move to
a store the instances share (Firestore, Redis), which is contained to
`rateLimiter` since nothing outside that file knows where a bucket lives.
Buckets that have refilled to full are dropped as they are passed, a minute at
a time: a full bucket admits exactly what an unknown key admits, so forgetting
it changes nothing, and the map stays bounded by active keys rather than by
every address ever seen.

## The writing assistant

The `/blogs/{slug}/chat` routes let an author talk to Gemini about a post and
have it make the changes itself, rather than handing back text to paste. Three
things are worth knowing about how it is bounded:

- **What it can touch.** The model is given three tools - `set_title`,
  `set_content`, `replace_text` - and they operate on an `entity.Draft`, which
  is a post's title and body and nothing else. Visibility, the private-post
  whitelist, the owner, and the slug are not reachable from any tool, so the
  worst a misbehaving model can do is write a bad post: it cannot publish a
  private one, reassign it, or touch a post other than the one being discussed.
  Nothing behind `repository.Assistant` writes at all - the adapter edits a copy
  and hands it back, and `service` decides whether to persist it.
- **Who may use it.** `entity.AssistantEntitlement` is the whitelist the access
  model gates it with, and the one whitelist here whose membership is not a
  field on a document: it is worked out per request from what the account has.
  An account is entitled while its subscription has not run out
  (`User.SubscribedUntil`), or unconditionally when the deployment granted its
  verified address (`ASSISTANT_ALLOWED_EMAILS`, configured per environment) -
  which is how every entitled account gets there today, since nothing sells a
  subscription yet. The two are the same answer to the routes; only the reason
  differs.

  A grant is matched against the caller's verified credential - never against
  anything the request asserts about itself, and never against an address the
  provider did not mark `email_verified`. It is keyed on the address rather than
  on the profile's username because a username is freely chosen and, once
  released, claimable by anybody: a list naming one would follow the name rather
  than the account. A subscription needs no such care, being keyed on the
  account already.

  The entitlement answers in the same shape as every other permission - a
  whitelist holding exactly one name, the caller's own, when they are entitled
  and nobody at all when they are not - so "may I use the assistant" is the same
  kind of question as "may I read this post", asked the same way. All three chat
  routes cost it: a transcript is as much a paid artifact as the turn that
  produced it. `GET /users/me` reports the answer as `assistantEnabled` (and the
  account's own `subscribedUntil` beside it), so a client can keep the panel off
  the screen for an account that would be refused; the routes enforce it either
  way, and a public profile never discloses either field.

  A deployment with no model configured is the zero entitlement: nobody is
  entitled, whatever anyone paid, because there is nothing for an entitlement to
  buy. Note that `SubscribedUntil` lives on the profile, so deleting a profile
  drops the subscription with it - which is fine while nothing sells one, and is
  the first thing to settle when a checkout does (either by refusing to delete a
  profile with live paid access, or by storing the subscription beside the
  profile rather than in it).
- **What the draft is.** A chat request carries the title and body the author
  has on screen, unsaved changes included - asking to tighten a paragraph has to
  mean the paragraph they can see, not the one last written to Firestore. The
  post is written back only when the model actually edited it, and the response
  says so (`updated`), so an author who only asked a question keeps their
  unsaved work unsaved.

Conversations are stored in Firestore under `chats/{slug}`, keyed exactly as the
post is. The turns live in an array inside that one document rather than in a
subcollection: a chat is only ever read whole, since the model is given the
whole history each turn. That trades unbounded growth for simplicity, which is
why `entity.MaxChatMessages` trims the oldest turns rather than failing a write.

The model is reached through the **Gemini Enterprise Agent Platform** - the
product formerly called Vertex AI, whose API is still served at
`aiplatform.googleapis.com` and whose discovery document now titles itself
"Agent Platform API". The backend sends a bearer token from Application Default
Credentials - on Cloud Run, its own runtime service account, holding
`roles/aiplatform.user` - so there is no long-lived credential to mint, store,
rotate, or leak, the same argument that put GitHub Actions on Workload Identity
Federation. Credentials are resolved on first use, so a backend started without
them still serves every other route and fails only this one, with the reason.

It is deliberately **not** the Gemini API (`generativelanguage.googleapis.com`),
and the reason is worth recording because it is not obvious: that API's
`generateContent` declares no OAuth scope at all in its discovery document, so
it takes an API key and nothing else. A service-account token sent to it is
refused with a `403`. This platform's `generateContent` does declare the
`cloud-platform` scope, which is what makes key-free authentication possible
here. Using the Gemini API instead would mean adding the one long-lived secret
this deployment is built to avoid.

When a call does fail, the adapter logs the provider's machine-readable `status`,
the `details[].reason` a denial carries (`PERMISSION_DENIED`,
`ACCESS_TOKEN_SCOPE_INSUFFICIENT`, `SERVICE_DISABLED`, ...), and the
`fieldViolations[].field` paths a rejected request carries. It never logs the
human-readable message, which can quote the request back, and the request holds
the post. A bare status code cannot tell an operator whether a deployment is
missing a role, a scope, an enabled API, or a well-formed request; those three
can.

One consequence of talking to a thinking model is worth knowing about. A turn
that has to be replayed into the conversation - which is every turn that makes a
tool call, since the call has to be in the transcript before its result can
answer it - is sent back as the exact bytes it arrived as, not re-encoded from
the fields `part` models. A thinking model returns parts this package has no
field for (`thought`, `thoughtSignature`, "an opaque signature for the thought so
it can be reused in subsequent requests"), and paraphrasing such a part through
a narrower struct turns it into `{}`, which the API rejects. The rule is
therefore not to model more fields but to stop paraphrasing what the model
said.

## Development

```sh
make install   # go mod download
make lint      # gofmt -l + go vet
make test      # go test ./... -cover
make build     # builds bin/backend
```

`go run ./cmd/backend` starts the server directly on `:8080`.

## Configuration

| Env var               | Description                                                                 |
| ---------------------- | ---------------------------------------------------------------------------- |
| `PORT`                 | Port to listen on. Defaults to `8080` (Cloud Run sets this itself).        |
| `ENVIRONMENT`          | Deployment environment reported by `/debug` (e.g. `stag`, `prod`). Defaults to `development`. |
| `CORS_ALLOWED_ORIGIN`  | Origin allowed to call this API from a browser (the frontend's URL). Unset disables CORS headers entirely. |
| `GOOGLE_CLIENT_ID`     | OAuth 2.0 client ID that ID tokens must be minted for. Set by Terraform from the `GOOGLE_CLIENT_ID` GitHub Actions variable; unset means no request can authenticate. |
| `GCP_PROJECT_ID`       | Project the model is called through and billed to. Unset disables the writing assistant. |
| `ASSISTANT_MODEL`      | Model id, e.g. `gemini-3.7-flash`. It has to be one the platform serves in `ASSISTANT_LOCATION`. Unset disables the writing assistant. |
| `ASSISTANT_LOCATION`   | Location the model is called in: a region such as `europe-west1`, or `global` for the multi-region endpoint. Defaults to `global`. |
| `ASSISTANT_ALLOWED_EMAILS` | Comma-separated verified account addresses permitted to use the writing assistant. Unset enables it for nobody. |

`commit` is not an env var — it's baked into the binary at build time via
`-ldflags "-X main.commit=..."` (see `Dockerfile`), since the same image is
promoted unmodified from staging to production.
