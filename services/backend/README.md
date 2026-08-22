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
