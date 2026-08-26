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
    firestore/        Firestore implementations of BlogRepository and UserRepository
    google/           Google Identity Services implementation of TokenVerifier
  service/            HTTP server: routes, middleware, and logic spanning more than one entity
```

Dependencies point inward: `service` and `repository` both know `entity`, `entity` knows nothing
else, and only `cmd/backend` knows which concrete adapters exist. Swapping Firestore for another
store means adding a folder under `repository/` and changing one line in `cmd/backend`.

Validation lives on the entities as setters (`SetDisplayName`, `SetVisibility`, ...) that trim,
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
  private posts are never fetched; `GET /blogs/{id}` applies it to the loaded
  document. Both routes run `optionalAuth`: a signed-in caller's own private
  and whitelisted posts are included, but no credential is required.
- **Writes** — `requireOwnedBlog` restricts updates and deletes to the post's
  owner, and `createdAt`/`updatedAt` come from the server rather than a
  spoofable client clock.

Profiles in `/users/{userId}` are keyed by the owner's Google account ID, so
there is no server-assigned ID to hand out and a profile is written with `PUT`
rather than `POST`. Any signed-in caller may read a profile; `requireSelf`
restricts writes to the profile's own owner.

Every route below except `GET /blogs` and `GET /blogs/{id}` requires a Google
Sign-In credential — those two run `optionalAuth` instead of `requireAuth`, so
an anonymous request is answered as far as public posts allow rather than
rejected. A signed-in caller sends the ID token Google issued, plus a header
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
| GET    | `/blogs/{id}` | Fetch a single blog. Caller must be allowed to read it; no credential required for a public one. |
| POST   | `/blogs`      | Create a blog. `ownerId` is always the caller, regardless of the body. |
| PUT    | `/blogs/{id}` | Replace a blog's fields. Caller must be the owner.                     |
| DELETE | `/blogs/{id}` | Delete a blog. Caller must be the owner.                               |
| GET    | `/users/{id}` | Fetch a profile. Readable by any signed-in caller.                     |
| PUT    | `/users/{id}` | Create or replace your own profile. `id` must be the caller's uid.     |
| DELETE | `/users/{id}` | Delete your own profile. `id` must be the caller's uid.                |

### Response shape

Successful responses return the resource itself (`201 Created` the first time
a profile is `PUT`, `200 OK` thereafter; `204 No Content` for `DELETE`). Every non-2xx response is JSON of the same shape, so a client can
parse success and failure the same way:

```json
{ "error": "blog not found" }
```

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

`commit` is not an env var — it's baked into the binary at build time via
`-ldflags "-X main.commit=..."` (see `Dockerfile`), since the same image is
promoted unmodified from staging to production.
