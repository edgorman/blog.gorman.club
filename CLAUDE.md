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

### Staging Deployments (Push to main)

Merging a pull request to `main` automatically deploys backend services to Cloud Run in the `my-app-staging` project and updates the staging subdomain on Cloudflare Pages.

### Pre-Release Generation

Merges to `main` calculate an incremental pre-release tag (e.g., `v1.0.0-rc.1`), execute a `terraform plan` against the production environment, and auto-generate a GitHub Pre-Release. This entry includes the plan output and a summary of commits merged since the last release. Multiple pre-releases can accumulate on `main` without touching production.

### Production Releases (Manual Promotion)

Promoting a specific pre-release tag via GitHub Actions converts it into a formal Release (e.g., `v1.0.0`). This triggers `terraform apply` on `my-app-prod`, updating infrastructure state and shifting Cloud Run traffic to the corresponding release image tag tested in staging.

### Rollback Strategy

Promoting a previous release tag executes `terraform apply` using the prior stable configuration and container tags.

### Pipeline Safety & Reusability

- **Concurrency Controls** — Every pipeline enforces an environment-scoped concurrency group to prevent overlapping commits from executing concurrent `terraform apply` steps or corrupting state locks.
- **Composite Actions** — Common pipeline steps (such as running Terraform plans/applies or building and pushing Docker images) are extracted into local GitHub Composite Actions inside `/.github/actions/` to keep root workflow files DRY and maintainable.

## Health Verification Strategy

Before building application features, setup is validated using a lightweight Debug Endpoint Contract:

- **Backend** — Exposes a `/health` or `/debug` endpoint returning system status, timestamp, environment metadata, and git commit SHA.
- **Frontend** — A dashboard fetches the backend `/debug` endpoint on page load.
- **Outcome** — Green indicators on `staging.example.com` and `example.com` verify that cross-cloud DNS routing, CORS policies, environment variables, and project-isolated IAM bindings are fully operational.
