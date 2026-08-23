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

## Users and blogs

Data lives in Firestore (`infrastructure/env/firestore.tf` and
`firestore.rules`). Responsibility is split so that no access rule is
implemented twice:

- **Reads** — the frontend queries users and blogs **directly through the
  Firebase SDK**. `firestore.rules` is the only place the read rules
  (public / owner / `allowedUserIds` whitelist) are expressed. This service
  has no read endpoints, so there's no Go copy of that logic to drift.
- **Writes** — blog writes go through this service so `createdAt` and
  `updatedAt` come from the server rather than a spoofable client clock.
  Because the Admin SDK used here bypasses `firestore.rules`, ownership is
  checked in `requireOwnedBlog` — that's the enforcement point for this path,
  not a duplicate of the read rules.

User profiles have no endpoints at all: `firestore.rules` already restricts
writes to the profile's own owner, so a server hop would add nothing.

Every route below requires a valid Firebase Auth ID token as a bearer token:

```
Authorization: Bearer <firebase-id-token>
```

| Method | Path          | Description                                                           |
| ------ | ------------- | --------------------------------------------------------------------- |
| POST   | `/blogs`      | Create a blog. `ownerId` is always the caller, regardless of the body. |
| PUT    | `/blogs/{id}` | Replace a blog's fields. Caller must be the owner.                     |
| DELETE | `/blogs/{id}` | Delete a blog. Caller must be the owner.                               |

### Response shape

Successful responses return the resource itself (or `204 No Content` for
`DELETE`). Every non-2xx response is JSON of the same shape, so a client can
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

`go run .` starts the server directly on `:8080`.

## Configuration

| Env var               | Description                                                                 |
| ---------------------- | ---------------------------------------------------------------------------- |
| `PORT`                 | Port to listen on. Defaults to `8080` (Cloud Run sets this itself).        |
| `ENVIRONMENT`          | Deployment environment reported by `/debug` (e.g. `stag`, `prod`). Defaults to `development`. |
| `CORS_ALLOWED_ORIGIN`  | Origin allowed to call this API from a browser (the frontend's URL). Unset disables CORS headers entirely. |

`commit` is not an env var — it's baked into the binary at build time (see
`Dockerfile`), since the same image is promoted unmodified from staging to
production.
