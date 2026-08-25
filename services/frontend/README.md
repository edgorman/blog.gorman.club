# frontend

Vite/React single-page app for blog.gorman.club, deployed to Cloudflare Pages
(see the repository root `CLAUDE.md` for the full deployment architecture).

It is an engineering console for the backend API, not a public-facing blog:
four panels that exercise every endpoint end to end, so a green page proves
cross-cloud routing, CORS, and token verification all work.

- **Backend status** — calls `/debug` (the Debug Endpoint Contract described
  in `CLAUDE.md`) on page load.
- **Sign in** — Google Sign-In, showing the id, email, and name the credential asserts.
- **Your profile** — `GET`/`PUT`/`DELETE /users/{id}` for the signed-in caller.
- **Blogs** — list, create, update, and delete against `/blogs`, with write
  buttons shown only for posts the caller owns.

Every panel degrades to an explanatory message rather than an error when its
configuration is missing, so the page is still useful before anything is
deployed.

## Google Sign-In

Sign-in uses [Google Identity Services](https://developers.google.com/identity/gsi/web/guides/overview)
(GSI). There is no login endpoint and no session: `index.html` loads the GSI
client library, `src/hooks/useGoogleAuth.ts` initialises it and renders the
button, and on sign-in Google hands back a signed JWT credential. The hook
decodes that credential client-side **for display only** and reuses the raw
string as the bearer credential on every backend request, alongside an
`Authorization-Provider: google` header. The backend re-verifies it per request
against Google's public keys, so nothing the browser decodes is trusted.

Because there is no server-side session, signing out is purely client-side
(clear local state, `disableAutoSelect()`), and a refresh signs you out.

### 1. Create an OAuth 2.0 client ID

Follow [Get your Google API client ID](https://developers.google.com/identity/gsi/web/guides/get-google-api-clientid):

1. Open the [credentials page](https://console.cloud.google.com/apis/credentials)
   for the **`blog-gorman-club-root`** project — the client ID is defined once in
   `infrastructure/root` and reused by every environment, so it belongs there
   rather than in the stag/prod projects.
2. Configure the OAuth consent screen if you haven't already.
3. **Create Credentials → OAuth client ID → Web application**.
4. Add every frontend URL that renders the button under **Authorized JavaScript
   origins**: `http://localhost:5173` for local dev, plus the staging and
   production site URLs.
5. Leave **Authorized redirect URIs** empty — GSI's ID-token flow returns the
   credential straight to the page's JS callback, with no server-side redirect.
6. Copy the **Client ID**. The client secret is not needed.

### 2. Configure it

Set `google_client_id` in `infrastructure/config/root/terraform.tfvars` and
apply `infrastructure/root`. That apply writes it into the `GOOGLE_CLIENT_ID`
GitHub Actions variable, and CI takes it from there for both environments with
no further per-environment configuration:

- the frontend Docker build gets it as `VITE_GOOGLE_CLIENT_ID`;
- `infrastructure/env` gets it as `TF_VAR_google_client_id`, which becomes the
  backend Cloud Run service's `GOOGLE_CLIENT_ID` — the audience it verifies
  tokens against.

For local development, put `VITE_GOOGLE_CLIENT_ID` in
`services/frontend/.env.local` and set `GOOGLE_CLIENT_ID` in the backend's
environment, using the same client ID (with `http://localhost:5173` among its
authorized origins).

If the client ID is unset, the frontend shows a "not configured" message
instead of a button, and the backend answers any authenticated request with a
`500` rather than accepting it.

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
| `VITE_GOOGLE_CLIENT_ID`      | Google OAuth 2.0 client ID. Not a secret; see above.      |

Both are baked into the bundle at build time — the frontend has no runtime
configuration.
