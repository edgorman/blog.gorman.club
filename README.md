# blog.gorman.club

This repository is a demo of a simple, cloud-deployed web service — a small full-stack app with a CI/CD pipeline and infrastructure-as-code — rather than a real blog hosting platform.

## Repository Structure

- `/infrastructure` — Terraform manifests provisioning the GCP and Cloudflare resources for the root, staging, and production environments.
- `/services/backend` — A Golang backend service packaged as a Docker container and deployed to GCP Cloud Run.
- `/services/frontend` — A Vite/React single-page app deployed to Cloudflare Pages.
- `/.github/actions` — Reusable composite GitHub Actions shared across the CI/CD workflows.
- `/.github/workflows` — GitHub Actions workflows that build, test, and deploy the services and infrastructure.
