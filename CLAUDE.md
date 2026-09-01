# CLAUDE.md

This document describes the deployment architecture and operating conventions for this repository, for use by Claude Code and other contributors.

## Overview

This is a single-repository (monorepo), multi-cloud deployment strategy built on Trunk-Based Development. It manages a static frontend, containerized backend services, infrastructure code, and repository governance from one central location.

## Repository Structure

- `/infrastructure` — Centralized Terraform manifests. The `env` subfolder holds the manifests applied once per environment (staging/prod) using environment-specific variable configurations (`staging.tfvars`, `prod.tfvars`); the `root` subfolder holds shared, manually-bootstrapped resources (see Root Environment below).
- `/services/backend` — Golang backend service(s) packaged as Docker containers targeted for GCP Cloud Run.
- `/services/frontend` — Conventional Vite/React single-page app deployed to Cloudflare Pages.
- `/services/*/Makefile` — Per-service `lint` and `test` targets used by both local development and CI (to be added later).
- `/.github/actions` — Modular, local GitHub Composite Actions (`action.yml`) encapsulating reusable workflow logic.
- `/.github/settings.yml` — Repository settings, branch permissions, and rulesets managed declaratively as code via the Probot Settings App.
- `/.github/workflows` — Event-specific workflow YAMLs (e.g. pull request, commit, release) that use reusable GitHub Actions.

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

### AI Writing Assistant

The backend calls Gemini models on the **Gemini Enterprise Agent Platform** — the product formerly called Vertex AI, whose API is still served at `aiplatform.googleapis.com` (its discovery document is now titled "Agent Platform API"). It authenticates as the Cloud Run runtime service account (`roles/aiplatform.user`) over Application Default Credentials rather than with an API key, which is the same reasoning that put CI on Workload Identity Federation: there is no long-lived credential to store in Secret Manager, rotate, or leak.

The platform is chosen over the **Gemini API** (`generativelanguage.googleapis.com`) for exactly that reason. The Gemini API's `generateContent` declares no OAuth scope in its discovery document, so it accepts an API key and nothing else; a token-authorized request to it is refused with a `403`. Only this platform's `generateContent` declares the `cloud-platform` scope, and so accepts the credential the deployment already has. Reaching for the Gemini API instead would mean introducing the one long-lived secret this architecture is built to avoid.

The model id and its location are Terraform variables (`assistant_model`, `assistant_location`) rather than constants, since model availability is regional and model ids change faster than this service is redeployed.

Access is an allowlist of verified Google account addresses (`assistant_allowed_emails`), configured per environment and enforced by the backend on every assistant route. It is matched against the `email` claim of the ID token, and only when that token also asserts `email_verified` - an address an account merely claimed is never a match. It is keyed on the address rather than the username deliberately: a username is freely chosen and, once released, claimable by anybody, so a list naming one would follow the name rather than the account. It is otherwise the simplest thing that works, and is the seam a real entitlement - a tier and an expiry tied to a payment - replaces later, since every caller already asks the question in those terms.

### Resource Naming

Strict environment suffixes (`backend-stag`, `backend-prod`) and scoped secrets (`stag-db-pass` vs `prod-db-pass`) ensure services in staging cannot accidentally reach production resources.

## CI/CD, Branching, & Release Lifecycle

All build, test, infrastructure provisioning, and deployment pipelines run exclusively through GitHub Actions (bypassing GCP Cloud Build). Sequential execution rules ensure infrastructure provisioning completes successfully before service updates occur.

### Versioning

Releases use plain semantic versioning — `major.minor.patch`, with no `-rc.N` or other pre-release suffix on the tag itself (pre-release vs. formal release is tracked via GitHub's release "prerelease" flag, not the tag name). The default increment on every merge to `main` is a **patch** bump over the last tag; developers can rename the generated pre-release before promotion if a `minor` or `major` bump is warranted instead.

### Staging Deployments (Push to main)

Merging a pull request to `main` automatically builds backend container images and pushes them to the `blog-gorman-club-stag` Artifact Registry, tagged with the commit SHA (never the calculated version — that can still change if a developer renames the pre-release before promotion), then deploys that image to Cloud Run in the `blog-gorman-club-stag` project and updates the staging subdomain on Cloudflare Pages. The frontend is built as a Docker image the same way, tagged with the commit SHA and `latest` in its own `blog-gorman-club-stag` Artifact Registry repository; its static files are then extracted from that image and pushed to Cloudflare Pages, so nothing is rebuilt separately for the live staging site.

### Pre-Release Generation

Merges to `main` calculate the next version (see Versioning above), summarize commit messages since the last tag, execute a `terraform plan -lock=false` against the production environment, and auto-generate a GitHub Pre-Release. This entry includes the plan output and the commit summary. The plan is read-only and disables state locking so it never contends with an in-flight production `terraform apply`, and so a pre-release build on `main` can never block a promotion from applying. Multiple pre-releases can accumulate on `main` without touching production.

### Production Releases (Manual Promotion)

Promoting a specific pre-release tag via GitHub Actions converts it into a formal Release (e.g., `v1.0.0`) and runs, in order:

1. `terraform apply` on `blog-gorman-club-prod`, applying any infrastructure changes first.
2. Backend image promotion via [`gcrane`](https://github.com/google/go-containerregistry/tree/main/cmd/gcrane) — the workflow resolves the target commit SHA directly from the release tag pointer (`github.sha`), looks up that commit-SHA-tagged image in the `blog-gorman-club-stag` Artifact Registry, and copies it by digest into the `blog-gorman-club-prod` Artifact Registry, retagged with the formal release version (e.g. `v1.0.0`). Images are **never rebuilt** for production; promotion guarantees prod runs the identical bytes validated in staging.
3. Cloud Run traffic in `blog-gorman-club-prod` is bumped to the newly copied image tag, completing the release.
4. Frontend image promotion works differently, since `VITE_BACKEND_URL` is baked into the frontend at build time rather than read at runtime like the backend's config — a staging build can't simply be copied into prod, it would carry staging's backend URL with it. Instead the workflow checks whether an image already exists at this release's tag in the `blog-gorman-club-prod` Artifact Registry; if not (a genuinely new release), it builds one from source with prod's backend URL baked in and pushes it. Either way, that image's static files are then extracted and deployed to the production subdomain on Cloudflare Pages.

### Rollback Strategy

Promoting a previous release tag re-runs the same promotion flow: `terraform apply` using that release's infrastructure configuration, then shifting Cloud Run traffic back to its already-promoted backend image tag in `blog-gorman-club-prod` (no re-copy needed, since that version was promoted previously). The frontend's rollback is the same idea applied to its own build-once-per-release image: since that release's tag already exists in `blog-gorman-club-prod` Artifact Registry, the build is skipped and its already-existing static files are just redeployed to Cloudflare Pages — no rebuild either way.

### Pipeline Safety & Reusability

- **Concurrency Controls** — Every pipeline enforces an environment-scoped concurrency group to prevent overlapping commits from executing concurrent `terraform apply` steps or corrupting state locks.
- **Composite Actions** — Common pipeline steps (such as running Terraform plans/applies or building and pushing Docker images) are extracted into local GitHub Composite Actions inside `/.github/actions/` to keep root workflow files DRY and maintainable.

## Health Verification Strategy

Before building application features, setup is validated using a lightweight Debug Endpoint Contract:

- **Backend** — Exposes a `/health` or `/debug` endpoint returning system status, timestamp, environment metadata, and git commit SHA.
- **Frontend** — A dashboard fetches the backend `/debug` endpoint on page load.
- **Outcome** — Green indicators on `staging.example.com` and `example.com` verify that cross-cloud DNS routing, CORS policies, environment variables, and project-isolated IAM bindings are fully operational.

## Monitoring & Alerting

`/infrastructure/env/monitoring.tf` watches each environment, so staging and prod are observed identically and a policy that turns out to be noisy is discovered in staging first.

- **Uptime check** — Polls the backend's `/health` endpoint, matching on the `"status":"ok"` body rather than the 200 alone: a status code only proves something is listening on the hostname, the body proves the backend answered. The period is the longest one offered (15 minutes) on purpose — the service scales to zero, so every probe is a request that may cold-start an instance, and a minute-by-minute check would keep one warm around the clock for no diagnostic gain.
- **Alert policies** — Three, all on Cloud Run's own metrics rather than log-based metrics, so nothing has to be kept in step with the handlers: the uptime check failing from more than one region (a single failing checker is more likely to be that checker), more than N `5xx` responses in a five-minute window (counted rather than rated, since at this traffic a rate reads as noise), and 95th-percentile latency above a threshold set well clear of a cold start.
- **Notification channels** — One email channel per address in `alert_notification_emails`, kept separate rather than combined so a later policy can notify a subset without duplicating a channel. Unlike `assistant_allowed_emails` this list grants nothing; it is where a message goes, not an account admitted.

### Budget Alerts

A budget is the one monitoring resource that is not a project resource: it belongs to the billing account, so creating one needs `billing.budgets.create` there and no project-level role CI holds includes it. It therefore ships disabled (`budget_amount = 0`) and is switched on in two steps — the same shape of exception as the GitHub PAT, one grant the automation cannot make for itself because it is the thing being authorised:

1. Grant the CI service account `roles/billing.costsManager` on the billing account once, by hand (`gcloud billing accounts add-iam-policy-binding`; the exact command is in the `budget_amount` description in `/infrastructure/env/variables.tf`).
2. Set `budget_amount` in each environment's `terraform.tfvars`, in the billing account's own currency — the amount deliberately names no `currency_code`, since a budget must be denominated in the billing account's currency and naming one here could only ever disagree with it.

The billing account id itself is never written into this repository. Root reads it off the project it already imported and writes it to a `gcp_billing_account` secret in Secret Manager, which the environment workflows read into `TF_VAR_billing_account` through the same `gcp-secret-manager` action the root workflows already use for the PAT and the Cloudflare credentials. So unlike those, it is supplied by hand nowhere at all: root is told it by the project, and CI is told it by root.
