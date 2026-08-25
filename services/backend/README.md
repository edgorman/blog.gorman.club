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

Data lives in Firestore (`infrastructure/env/firestore.tf`). This service is
the only client of that database: it authenticates with the Admin SDK, no
Firestore security rules are deployed, and the browser never talks to
Firestore directly. Access rules therefore live in Go and nowhere else:

- **Reads** — `canRead` in `blogs.go` is the single definition of who may see
  a post (public posts for any signed-in caller, private ones for the owner
  or a uid in `allowedUserIds`). `GET /blogs` applies it as a Firestore query
  so private posts are never fetched; `GET /blogs/{id}` applies it to the
  loaded document.
- **Writes** — `requireOwnedBlog` restricts updates and deletes to the post's
  owner, and `createdAt`/`updatedAt` come from the server rather than a
  spoofable client clock.

Profiles in `/users/{userId}` are keyed by the owner's Firebase Auth uid, so
there is no server-assigned ID to hand out and a profile is written with `PUT`
rather than `POST`. Any signed-in caller may read a profile; `requireSelf`
restricts writes to the profile's own owner.

Every route below requires a valid Firebase Auth ID token as a bearer token:

```
Authorization: Bearer <firebase-id-token>
```

| Method | Path          | Description                                                           |
| ------ | ------------- | --------------------------------------------------------------------- |
| GET    | `/blogs`      | List the blogs the caller may read, newest first.                      |
| GET    | `/blogs/{id}` | Fetch a single blog. Caller must be allowed to read it.                |
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
