# CLAUDE.md

This document describes the deployment architecture and operating conventions for this repository, for use by Claude Code and other contributors.

## Overview

This is a single-repository (monorepo), multi-cloud deployment strategy built on Trunk-Based Development. It manages a static frontend, containerized backend services, infrastructure code, and repository governance from one central location.

## Repository Structure

- `/infrastructure` — Centralized Terraform manifests using environment-specific variable configurations (`staging.tfvars`, `prod.tfvars`).
- `/services/backend` — Backend service(s) packaged as Docker containers targeted for GCP Cloud Run.
- `/services/frontend` — Static single-page app deployed to Cloudflare Pages.
- `/.github/actions` — Modular, local GitHub Composite Actions (`action.yml`) encapsulating reusable workflow logic.
- `/.github/settings.yml` — Repository settings, branch permissions, and rulesets managed declaratively as code via the Probot Settings App.
- `/.github/workflow` — Event-specific workflow YAMLs (e.g. pull request, commit, release) that use reusable GitHub Actions.

## Cloud Infrastructure & Security Isolation

### Isolated GCP Projects

To maintain strict blast radius boundaries, environments live in distinct GCP projects (placeholder naming for now):

- `my-app-staging` — Hosts staging services and serves as the isolated sandbox for testing. Future feature: ephemeral/PR environments.
- `my-app-prod` — Hosts live production infrastructure and sensitive datastores.

### IAM & Authentication

Workflows authenticate to GCP using Workload Identity Federation (WIF) over short-lived OIDC tokens, avoiding long-lived JSON keys. Service accounts are bound strictly per project.

### Resource Naming

Strict environment suffixes (`backend-staging`, `backend-prod`) and scoped secrets (`staging-db-pass` vs `prod-db-pass`) ensure services in staging cannot accidentally reach production resources.

## CI/CD, Branching, & Release Lifecycle

All build, test, infrastructure provisioning, and deployment pipelines run exclusively through GitHub Actions (bypassing GCP Cloud Build). Sequential execution rules ensure infrastructure provisioning completes successfully before service updates occur.

### Versioning

Releases use plain semantic versioning — `major.minor.patch`, with no `-rc.N` or other pre-release suffix on the tag itself (pre-release vs. formal release is tracked via GitHub's release "prerelease" flag, not the tag name). The default increment on every merge to `main` is a **patch** bump over the last tag; developers can rename the generated pre-release before promotion if a `minor` or `major` bump is warranted instead.

### Staging Deployments (Push to main)

Merging a pull request to `main` automatically builds backend container images and pushes them to the `my-app-staging` Artifact Registry, tagged with the calculated version, then deploys them to Cloud Run in the `my-app-staging` project and updates the staging subdomain on Cloudflare Pages.

### Pre-Release Generation

Merges to `main` calculate the next version (see Versioning above), execute a `terraform plan` against the production environment, and auto-generate a GitHub Pre-Release. This entry includes the plan output and a summary of commits merged since the last release. Multiple pre-releases can accumulate on `main` without touching production.

### Production Releases (Manual Promotion)

Promoting a specific pre-release tag via GitHub Actions converts it into a formal Release (e.g., `v1.0.0`) and runs, in order:

1. `terraform apply` on `my-app-prod`, applying any infrastructure changes first.
2. Image promotion via [`gcrane`](https://github.com/google/go-containerregistry/tree/main/cmd/gcrane) — the exact container images already built and tested in the `my-app-staging` Artifact Registry are copied by digest into the `my-app-prod` Artifact Registry and retagged with the release version. Images are **never rebuilt** for production; promotion guarantees prod runs the identical bytes validated in staging.
3. Cloud Run traffic in `my-app-prod` is bumped to the newly copied image tag, completing the release.

### Rollback Strategy

Promoting a previous release tag re-runs the same promotion flow: `terraform apply` using that release's infrastructure configuration, then shifting Cloud Run traffic back to its already-promoted image tag in `my-app-prod` (no re-copy needed, since that version was promoted previously).

### Pipeline Safety & Reusability

- **Concurrency Controls** — Every pipeline enforces an environment-scoped concurrency group to prevent overlapping commits from executing concurrent `terraform apply` steps or corrupting state locks.
- **Composite Actions** — Common pipeline steps (such as running Terraform plans/applies or building and pushing Docker images) are extracted into local GitHub Composite Actions inside `/.github/actions/` to keep root workflow files DRY and maintainable.

## Health Verification Strategy

Before building application features, setup is validated using a lightweight Debug Endpoint Contract:

- **Backend** — Exposes a `/health` or `/debug` endpoint returning system status, timestamp, environment metadata, and git commit SHA.
- **Frontend** — A dashboard fetches the backend `/debug` endpoint on page load.
- **Outcome** — Green indicators on `staging.example.com` and `example.com` verify that cross-cloud DNS routing, CORS policies, environment variables, and project-isolated IAM bindings are fully operational.
