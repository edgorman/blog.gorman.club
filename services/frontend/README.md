# frontend

Vite/React single-page app for blog.gorman.club, deployed to Cloudflare Pages
(see the repository root `CLAUDE.md` for the full deployment architecture).

It is the public blog: a recent-posts feed with search and tag filtering, a
single post view rendering markdown, a per-author profile feed, and a markdown
editor for publishing.

- **Feed** (`/`) — the 10 most recent posts the caller can read (every public
  post, plus the signed-in caller's own private ones), across all authors. A
  search box and a tag filter narrow it, and both live in the query string
  (`/?tag=go&q=generics`) rather than in component state: that is what makes a
  filtered feed a place — it survives a reload, it can be linked to and shared,
  and the tag chips on every post point at it. They compose with each other, and
  neither can widen the feed: the backend applies both on top of the same read
  rules, so a search cannot surface a post the caller could not have scrolled
  to.
- **Post** (`/post/:slug`) — `GET /blogs/{slug}`, rendered from markdown to
  HTML. The slug addresses the post on its own, since slugs are unique across
  every author, so `lib/api.ts`'s `postPath` builds a link from it alone. The
  author beside it is who wrote the post, and links to their profile. Beneath it
  is the comment thread (`components/Comments.tsx`, `GET /blogs/{slug}/comments`):
  readable by whoever can read the post, signed out included, and writable by
  anyone signed in. A body is rendered as *text*, not markdown — a post is
  written by the author whose page it is, a comment by whoever happened to read
  it, and the safe rendering of a stranger's input is the one with no syntax in
  it. The post's tags sit under the author as links into the feed filtered by
  each (`components/TagList.tsx`); the same chips in a feed row are plain text
  instead, since a row is itself one big link and an anchor inside an anchor is
  not valid HTML. Delete is offered to a comment's own author and to the post's owner, who
  moderates their own post; the backend decides either way. Both the post and
  each comment carry a reaction bar (`components/ReactionBar.tsx`): five fixed
  emoji (👍 👎 ❤️ 😄 🎉), each a chip showing its count and whether you are in
  it - no picker, no custom emoji, so reacting is one click. The page's
  reactions - the post's and every comment's - are loaded in one request by
  `hooks/useReactions.ts`, since the API answers them together. The five are
  kept in step with the backend's own `entity.AllowedEmojis`; widening the set
  means changing both.
- **Edit post** (`/post/:slug/edit`) — the same editor over an existing post,
  saving via `PUT /blogs/{slug}`. Only the owner sees the form.
- **New post** (`/post/new`) — a single-pane markdown editor with a Preview
  toggle, publishing via `POST /blogs`. Requires sign-in. Tags are typed as one
  comma-separated line; `lib/tags.ts` only splits it, since normalizing a tag
  ("Web Dev" into "web-dev") is the backend's `entity.NormalizeTag` and doing it
  here too would be a second definition of the same rule. The literal outranks
  the `:slug` wildcard beside it, and the backend reserves `new` as a slug, so
  no post can be published at a path the editor already occupies.
- **Profile** (`/user/:username`) — that author's recent posts, plus their
  username and bio from `GET /users/{username}`.
- **Edit profile** (`/user/:username/edit`) — the signed-in caller's own
  username and bio, saved via `PUT /users/me`. The path names a profile but the
  credential decides whose is written, so following somebody else's edit link is
  refused rather than silently applied to your own.

Every page degrades to an explanatory message rather than an error when its
configuration is missing, so the page is still useful before anything is
deployed.

`GET /users/{username}` admits anonymous callers, and a post carries its
author's username resolved server-side, so a signed-out visitor sees the same
authors and bios a signed-in one does. Only what is *readable* differs: private
posts appear once their owner or a whitelisted caller signs in.

## Google Sign-In

Sign-in uses [Google Identity Services](https://developers.google.com/identity/gsi/web/guides/overview)
(GSI). There is no login endpoint and no session: `index.html` loads the GSI
client library, `src/hooks/useGoogleAuth.ts` initialises it and renders the
button, and on sign-in Google hands back a signed JWT credential. The hook
decodes that credential client-side **for display only** and reuses the raw
string as the bearer credential on every backend request, alongside an
`Authorization-Provider: google` header. The backend re-verifies it per request
against Google's public keys, so nothing the browser decodes is trusted.

There is no server-side session, so staying signed in across a reload is
handled in two layers:

1. **The credential is cached in `sessionStorage`** and restored on mount if it
   hasn't expired. This is the deterministic path — it doesn't depend on
   Google's session state at all. `sessionStorage` rather than `localStorage`
   so the credential dies with the tab, keeping the window in which an XSS bug
   could read it as small as possible while still surviving a reload. An
   expired entry is discarded rather than restored, since the backend would
   reject it and the UI would look signed in while every request 401s.
2. **`prompt()` asks Google to reissue one** when nothing valid was restored (a
   new tab, or the cached credential aged out). With `auto_select` that is
   silent where Google permits it — which needs a single active Google
   session, prior consent, and under FedCM no more than one sign-in attempt in
   the preceding 10 minutes. Where it isn't permitted, One Tap appears and the
   rendered button remains as the explicit fallback.

`prompt()` is what makes `auto_select` do anything: it's a One Tap setting, so
initialising with `auto_select` while only ever calling `renderButton()` leaves
it inert.

Signing out clears the cache and calls `disableAutoSelect()`, so an explicit
sign-out isn't undone by either layer on the next load.

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
