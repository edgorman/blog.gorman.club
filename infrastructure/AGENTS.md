# infrastructure

Terraform roots: `env` (applied per environment) and `root` (shared, bootstrapped once). How these are planned and applied is in `.github/AGENTS.md`.

## Cloud Infrastructure & Security Isolation

### Isolated GCP Projects

To maintain strict blast radius boundaries, environments live in distinct GCP projects:

- `blog-gorman-club-stag` — Hosts staging services and serves as the isolated sandbox for testing. Future feature: ephemeral/PR environments.
- `blog-gorman-club-prod` — Hosts live production infrastructure and sensitive datastores.

### Root Environment

A third, non-application Terraform root lives at `/infrastructure/root` and provisions the `blog-gorman-club-root` GCP project itself, along with the resources shared across staging and prod: the `blog-gorman-club-stag` and `blog-gorman-club-prod` GCP projects, the Terraform state buckets for all environments (root included), the GitHub Actions WIF pool/provider, and any domain configuration shared between the two environments (e.g. the parent DNS zone). It also enables the baseline set of GCP APIs uniformly across root and both environment projects, so the per-environment Terraform roots can assume those APIs are already on.

Only root's very first apply is manual — it has to exist before any pipeline has credentials or state to work with, so it can't be bootstrapped by the CI/CD it enables. Because the root project's own state bucket doesn't exist until root has been applied once, that first `terraform apply` runs against local state; once the GCS bucket it creates exists, state is migrated into it (`terraform init -migrate-state`) and the local state files are discarded.

Every apply after that one-time bootstrap goes through the ordinary CI/CD flow, same as `blog-gorman-club-stag` and `blog-gorman-club-prod`: a `pull-request` workflow plans against `/infrastructure/root` for PRs that touch it, and a `push-commit` workflow applies it on merge to `main`, both authenticating with the WIF identity that bootstrap itself created.

### IAM & Authentication

Workflows authenticate to GCP using Workload Identity Federation (WIF) over short-lived OIDC tokens, avoiding long-lived JSON keys. A single GitHub Actions service account is created in the root environment and granted IAM roles on each project (root, staging, prod) individually — never a broad org-level grant — and the WIF provider's attribute condition restricts it to OIDC tokens asserting this specific GitHub repository. The resulting Workload Identity Provider path and service account email are written into GitHub Actions repository variables by the root apply, so workflows never hardcode them.

The one credential WIF can't replace is the GitHub PAT root's own Terraform needs to write those repository variables in the first place. It's supplied by hand only once, at bootstrap; every apply after that (including CI's) reads it back from a `github_provider_token` secret in GCP Secret Manager, fetched at the start of each workflow run using the WIF identity above — so it's never stored as a GitHub Actions secret.

### Resource Naming

Strict environment suffixes (`backend-stag`, `backend-prod`) and scoped secrets (`stag-db-pass` vs `prod-db-pass`) ensure services in staging cannot accidentally reach production resources.

### Artifact Stores

Every service has one store per environment project rather than one shared store copied between them: the backend publishes to that project's own Artifact Registry `backend` repository, and the frontend to that project's own `<gcp-project-id>-frontend` Cloud Storage bucket (`infrastructure/env/storage.tf`) - named after the project rather than just the environment because bucket names are global across GCS, unlike the project-scoped resources named elsewhere in this document. The GitHub Actions service account is granted write on both stores in staging and prod alike (per-project, per the IAM section above, never a broad grant), so either can be published straight into every environment's store from the job that builds and validates it.

Both services are there now: Staging Deployments (in `.github/AGENTS.md`) publishes the SHA-tagged backend image and the SHA-folder frontend build to both stores on every merge, and Production Releases (in `.github/AGENTS.md`) deploys straight from `blog-gorman-club-prod`'s own store for each service rather than copying into it. Neither promotion reads across the project boundary or copies anything - each project's store is written once, by the job that built and validated it, and only read from within that same project.

Every merge writes to both projects' stores regardless of whether that merge is ever promoted (see Staging Deployments in `.github/AGENTS.md`), so a store's size tracks merge volume, not deploy volume - prod's own registry and bucket fill up from commits even across a long gap between promotions. Nothing deleted any of it until now, so both stores grow forever. `infrastructure/env/artifact_registry.tf` and `storage.tf` each carry one cleanup rule, applied identically in both projects since neither store's retention need differs by environment:

- The backend repository's `cleanup_policies` keep the `backend_registry_keep_count` (default **30**) most recently uploaded versions and delete everything else - Artifact Registry's documented pattern for "keep only the N most recent," a KEEP policy pinning the most recent N versions combined with an unconditional DELETE policy for the rest. Thirty is sized off versions rather than days on purpose: a rollback goes back by release, not by calendar time, and thirty covers even this repository's busiest stretch of development (a dozen-odd merges in a single day) several times over, while a normal cadence of a few merges a week reaches back months. There is no cleanup policy condition for "a version a Cloud Run revision still references" - Artifact Registry cannot express that - so the guarantee is indirect: as long as prod is promoted well within every 30 merges to `main`, the image any Cloud Run revision is running always falls inside the kept window. If a gap ever approaches that, promote (or raise `backend_registry_keep_count`) before it closes.
- Each frontend bucket's `lifecycle_rule` deletes commit-SHA folders older than `frontend_retention_days` (default **90**) days. GCS lifecycle conditions are age-based only - there is no "keep the N most recent objects" equivalent for distinct object names the way Artifact Registry has one for versions - so this cannot single out "the folder currently deployed to prod" independent of how long ago it was promoted the way the registry's keep-count can single out recent versions by count. Ninety days is chosen generously against that: it comfortably outlasts any promotion gap seen in this repository so far, but the same operational rule applies - promote (or raise `frontend_retention_days`) before a live folder's age approaches the window, since nothing here can tell a live folder from an old one by age alone.

Both are dry-run before they can delete anything for real: the registry ships with `cleanup_policy_dry_run = true` (Artifact Registry's own dry-run mode - it evaluates and logs what the policy would delete without deleting it), and the bucket rule has no such built-in mode, so it needs a one-off listing of what it would currently match (`gcloud storage ls -L` filtered on creation time against `frontend_retention_days`) reviewed before it is relied upon. Enabling real deletion (flipping `cleanup_policy_dry_run` to `false`) is a deliberate follow-up once the dry-run output has been reviewed against what is actually serving each environment, not part of adding the policy itself.

## Monitoring & Alerting

`/infrastructure/env/monitoring.tf` watches each environment, so staging and prod are observed identically and a policy that turns out to be noisy is discovered in staging first.

- **Uptime check** — Polls the backend's `/health` endpoint, matching on the `"status":"ok"` body rather than the 200 alone: a status code only proves something is listening on the hostname, the body proves the backend answered. The period is the longest one offered (15 minutes) on purpose — the service scales to zero, so every probe is a request that may cold-start an instance, and a minute-by-minute check would keep one warm around the clock for no diagnostic gain.
- **Alert policies** — Three, all on Cloud Run's own metrics rather than log-based metrics, so nothing has to be kept in step with the handlers: the uptime check failing from more than one region (a single failing checker is more likely to be that checker), more than N `5xx` responses in a five-minute window (counted rather than rated, since at this traffic a rate reads as noise), and 95th-percentile latency above a threshold set well clear of a cold start.
- **Notification channels** — One email channel per address in `alert_notification_emails`, kept separate rather than combined so a later policy can notify a subset without duplicating a channel. This list grants nothing; it is where a message goes, not an account admitted.
