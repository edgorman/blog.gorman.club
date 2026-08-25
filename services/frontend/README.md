# frontend

Vite/React single-page app for blog.gorman.club, deployed to Cloudflare Pages
(see the repository root `CLAUDE.md` for the full deployment architecture).

It is an engineering console for the backend API, not a public-facing blog:
four panels that exercise every endpoint end to end, so a green page proves
cross-cloud routing, CORS, and token verification all work.

- **Backend status** — calls `/debug` (the Debug Endpoint Contract described
  in `CLAUDE.md`) on page load.
- **Sign in** — Google sign-in via Firebase Auth, showing the resulting uid.
- **Your profile** — `GET`/`PUT`/`DELETE /users/{id}` for the signed-in caller.
- **Blogs** — list, create, update, and delete against `/blogs`, with write
  buttons shown only for posts the caller owns.

Every panel degrades to an explanatory message rather than an error when its
configuration is missing, so the page is still useful before anything is
deployed.

## Firebase Auth

Firebase is used **only** to mint the ID token the backend verifies. No
Firestore access happens in the browser — there are no security rules
deployed, and every read and write goes through the API. Nothing here imports
`firebase/firestore`.

The web API key is not a secret (access is governed by the backend, not by
hiding it), so the config below lives in GitHub Actions *variables* rather
than Secret Manager. Values are baked in at build time, like
`VITE_BACKEND_URL`.

One-time setup per environment, in the Firebase console for
`blog-gorman-club-stag` / `blog-gorman-club-prod`:

1. Register a Web App and copy its `apiKey` and `authDomain`.
2. Enable the Google sign-in provider under **Authentication → Sign-in method**.
3. Add the site's domain under **Authentication → Settings → Authorized domains**.
4. Set the repository variables `FIREBASE_API_KEY_STAG` /
   `FIREBASE_AUTH_DOMAIN_STAG` (and the `_PROD` pair). The project ID is
   passed by the workflows directly.

Steps 1-3 are console-only because enabling a sign-in provider through
Terraform requires Identity Platform rather than the Firebase Auth free tier.

For local development, put the same values in `services/frontend/.env.local`
(already gitignored by Vite's defaults).

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

| Env var                     | Description                                              |
| ---------------------------- | --------------------------------------------------------- |
| `VITE_BACKEND_URL`           | Base URL of the backend service. Unset in local dev; set automatically in CI from the deployed Cloud Run service's URL. |
| `VITE_FIREBASE_API_KEY`      | Firebase web API key. Not a secret; see above.            |
| `VITE_FIREBASE_AUTH_DOMAIN`  | Firebase auth domain, e.g. `blog-gorman-club-stag.firebaseapp.com`. |
| `VITE_FIREBASE_PROJECT_ID`   | Firebase/GCP project ID for the environment.              |

All four are baked into the bundle at build time — the frontend has no runtime
configuration.
