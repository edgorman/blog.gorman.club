# frontend

Vite/React single-page app for blog.gorman.club, deployed to Cloudflare Pages
(see the repository root `CLAUDE.md` for the full deployment architecture).

The landing page renders a "Backend status" card that calls the backend's
`/debug` endpoint (the Debug Endpoint Contract described in `CLAUDE.md`) on
page load. Until a backend is deployed and `VITE_BACKEND_URL` is set, the
card renders a placeholder instead of making a network call.

## Development

```sh
make install   # npm ci
make lint      # oxlint
make test      # vitest --run
make build     # tsc -b && vite build
```

`npm run dev` starts the Vite dev server directly.

## Configuration

| Env var             | Description                                              |
| -------------------- | --------------------------------------------------------- |
| `VITE_BACKEND_URL`   | Base URL of the backend service. Unset in local dev / until the backend exists. |
