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
    firestore/        Firestore implementations of BlogRepository, UserRepository, ChatRepository
    google/           Google Identity Services implementation of TokenVerifier
    vertex/           Gemini-on-Vertex-AI implementation of Assistant
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

## Users and blogs

Data lives in Firestore (`infrastructure/env/firestore.tf`). This service is
the only client of that database: it authenticates with the Admin SDK, no
Firestore security rules are deployed, and the browser never talks to
Firestore directly. Access rules therefore live in Go and nowhere else:

- **Reads** — `Blog.CanBeReadBy` is the single definition of who may see a post
  (public posts for anyone, signed in or not; private ones for the owner or a
  uid in `allowedUserIds`). `GET /blogs` applies it as a Firestore query so
  private posts are never fetched; `GET /blogs/{slug}` applies it to
  the loaded document. Both routes run `optionalAuth`: a signed-in caller's own private
  and whitelisted posts are included, but no credential is required.
- **Writes** — `requireOwnedBlog` restricts updates and deletes to the post's
  owner, and `createdAt`/`updatedAt` come from the server rather than a
  spoofable client clock.

A post's address is a slug taken from its own title - so unlike the random key a
Firestore-assigned id would have been, it is guessable. `blogFromPath` and
`requireReadableBlog` (`internal/service/blog.go`) answer every way of missing - a
malformed slug, a slug nothing holds, and a post the caller is not allowed to read -
with the same `404`. A `403` would tell a prober a private post exists at a path it
merely guessed right; folding that case into "not found" is what keeps the guess from
confirming anything. `requireOwnedBlog` layers ownership on
top of readability: a caller who cannot read a post gets the masking `404` before
ownership is even considered, and only a post they can see yields a `403` for not
owning it - which discloses nothing they could not already see.

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

Every route below except `GET /blogs`, `GET /blogs/{slug}` and
`GET /users/{username}` requires a Google Sign-In credential — those three run
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

### Response shape

Successful responses return the resource itself (`201 Created` the first time
a profile is `PUT`, `200 OK` thereafter; `204 No Content` for `DELETE`). Every non-2xx response is JSON of the same shape, so a client can
parse success and failure the same way:

```json
{ "error": "blog not found" }
```

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
- **Who may use it.** `entity.AssistantAllowlist` is a static list of usernames,
  configured per environment (`ASSISTANT_ALLOWED_USERNAMES`). It is a type of
  its own rather than a string comparison in a handler because it is the seam
  the real thing replaces: when access becomes something bought, an entitlement
  carrying a tier and an expiry takes its place and `Allows` becomes a lookup
  that can also answer "expired". Every caller already asks the question in
  those terms. `GET /users/me` reports the answer as `assistantEnabled`, so a
  client can keep the panel off the screen for an account that would be refused;
  the routes enforce it either way, and a public profile never discloses it.
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

Gemini is reached through **Vertex AI**, not the Generative Language API, so the
backend authenticates as its own Cloud Run runtime service account
(`roles/aiplatform.user`, see `infrastructure/env/cloud_run.tf`) over
Application Default Credentials. There is no API key to mint, store, rotate, or
leak - the same argument that put GitHub Actions on Workload Identity
Federation. Credentials are resolved on first use, so a backend started without
them still serves every other route and fails only this one, with the reason.

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
| `GCP_PROJECT_ID`       | Project Vertex AI is called through and billed to. Unset disables the writing assistant. |
| `ASSISTANT_MODEL`      | Vertex model id, e.g. `gemini-3.7-flash`. It has to be one Vertex serves in `ASSISTANT_LOCATION`. Unset disables the writing assistant. |
| `ASSISTANT_LOCATION`   | Vertex location: a region such as `europe-west1`, or `global` for the multi-region endpoint. Defaults to `global`. |
| `ASSISTANT_ALLOWED_USERNAMES` | Comma-separated usernames permitted to use the writing assistant. Unset enables it for nobody. |

`commit` is not an env var — it's baked into the binary at build time via
`-ldflags "-X main.commit=..."` (see `Dockerfile`), since the same image is
promoted unmodified from staging to production.
