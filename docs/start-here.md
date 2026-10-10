# Start here

From a fresh clone to an open pull request. None of it needs GCP access.

## 1. Clone and install the toolchain

You need git, [proto](https://moonrepo.dev/docs/proto/install), and for the local backend the [gcloud CLI](https://cloud.google.com/sdk/docs/install) with Java 21 (no Google account or login needed).

```sh
git clone https://github.com/edgorman/blog.gorman.club.git
cd blog.gorman.club
proto install                                      # moon, Go, Node, npm, buf, Terraform at the pinned versions
gcloud components install cloud-firestore-emulator
```

## 2. Run the backend locally

```sh
gcloud emulators firestore start --host-port=127.0.0.1:8081   # terminal 1
CORS_ALLOWED_ORIGIN=http://localhost:5173 moon run services/backend:dev-local   # terminal 2, serves :8080
```

`dev-local` uses the Firestore emulator and a development sign-in token instead of Google's; `CORS_ALLOWED_ORIGIN` lets the frontend dev server call it. If `gcloud components install` isn't available, CI's route works too: download the emulator jar named in `pull-request.yaml`'s "Start the Firestore emulator" step and run `java -jar <jar> --host=127.0.0.1 --port=8081`.

Try it:

```sh
auth=(-H 'Authorization: Bearer dev' -H 'Authorization-Provider: google' -H 'Content-Type: application/json')
curl "${auth[@]}" -X PUT -d '{"username":"dev"}' localhost:8080/users/me
curl "${auth[@]}" -X POST -d '{"title":"Hello","content":"First post","visibility":"public"}' localhost:8080/blogs
curl localhost:8080/blogs
```

The emulator starts empty each time, and with no model configured the writing assistant is off and search is a plain text match. More in "Running locally" in `services/backend/README.md`.

## 3. Run the frontend

```sh
cd services/frontend
echo 'VITE_BACKEND_URL=http://localhost:8080' > .env.local    # use the local backend
cd ../..
moon run services/frontend:dev                               # Vite on http://localhost:5173
```

The feed shows the post you just created. Signing in from the browser needs a Google client ID (see `services/frontend/README.md`); nothing else does.

## 4. Run what CI runs

```sh
git fetch origin main
moon ci --base origin/main
```

This runs every check your branch affects. On a fresh branch nothing is affected yet; to run everything once, use `moon run :format :lint :vet :typecheck :build :test`. With `FIRESTORE_EMULATOR_HOST=127.0.0.1:8081` set, the backend tests also run the Firestore adapter tests against the emulator, as CI does. `infra-*:validate` needs Terraform's registry and `services/backend:image` needs Docker; CI runs both if you don't.

## 5. Make one change

- Read the root [`AGENTS.md`](../AGENTS.md) and the `AGENTS.md` of the area you're changing. They're short rules; each links to the decision record in [`decisions/`](decisions/README.md) that explains it.
- Changing the API shape? Edit the `.proto` in `packages/protos`, run `buf generate` there, and commit the generated code with it.
- Run step 4 again.

## 6. Open a pull request

- Start from an issue (see [`CONTRIBUTING.md`](../.github/CONTRIBUTING.md)).
- Title it as a Conventional Commit, e.g. `fix(frontend): keep the draft when sign-in expires`. The title becomes the commit on `main` and decides the version.
- Fill in the PR template. CI runs the checks from step 4, plus the title check and `buf breaking`. A code owner reviews, and on merge the change deploys to staging. Production is a separate promotion; nothing is deployed by hand.
