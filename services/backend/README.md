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

Data is stored in Firestore (`infrastructure/env/firestore.tf` and
`firestore.rules`). Since this service uses the Firestore Admin SDK - which
bypasses `firestore.rules` entirely - the same access control those rules
define is re-enforced in Go (see `Blog.visibleTo` in `model.go`, and the
handlers in `users.go`/`blogs.go`).

Every route below requires a valid Firebase Auth ID token as a bearer token:

```
Authorization: Bearer <firebase-id-token>
```

| Method | Path          | Description                                                              |
| ------ | ------------- | ------------------------------------------------------------------------- |
| GET    | `/users/{id}` | Get a user's profile. Any signed-in caller may read any profile.          |
| PUT    | `/users/{id}` | Create or replace a user's own profile. `id` must match the caller's uid. |
| GET    | `/blogs`      | List blogs visible to the caller (public, owned, or whitelisted).         |
| POST   | `/blogs`      | Create a blog. `ownerId` is always the caller, regardless of the body.    |
| GET    | `/blogs/{id}` | Get a blog if it's visible to the caller (404 otherwise, same as missing).|
| PUT    | `/blogs/{id}` | Replace a blog's fields. Caller must be the owner.                        |
| DELETE | `/blogs/{id}` | Delete a blog. Caller must be the owner.                                  |

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
