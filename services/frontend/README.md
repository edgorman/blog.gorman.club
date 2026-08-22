# frontend

Vite/React single-page app for blog.gorman.club, deployed to Cloudflare Pages
(see the repository root `CLAUDE.md` for the full deployment architecture).

The landing page renders a "Backend status" card that calls the backend's
`/debug` endpoint (the Debug Endpoint Contract described in `CLAUDE.md`) on
page load. `VITE_BACKEND_URL` is baked in at build time by the
`services-frontend` deploy job (see `push-commit.yaml` / `promote-release.yaml`),
which looks up the deployed Cloud Run service's URL. Locally, or if that
lookup comes back empty, the card renders a placeholder instead of making a
network call.

## Development

```sh
make install   # npm ci
make lint      # oxlint
make test      # vitest --run
make build     # tsc -b && vite build
```

`npm run dev` starts the Vite dev server directly.

CI builds the `Dockerfile` here instead of running `make build` directly, then extracts its
static files for the Cloudflare Pages deploy - so what's live always matches an image already
sitting in Artifact Registry, and a rollback can redeploy that image's files without a rebuild.

## Configuration

| Env var             | Description                                              |
| -------------------- | --------------------------------------------------------- |
| `VITE_BACKEND_URL`   | Base URL of the backend service. Unset in local dev; set automatically in CI from the deployed Cloud Run service's URL. |
