# 28. Each environment has its own artifact stores

Status: Accepted

## Context

A shared store copied between environments would mean a cross-project read or a copy at promotion time.

## Decision

Each environment project has its own Artifact Registry `backend` repository and its own `<gcp-project-id>-frontend` bucket (`infrastructure/env/storage.tf`; bucket names are global, so they're named after the project). The CI service account can write both stores in both projects, so the job that builds and validates an artifact publishes it into every environment's store ([22](0022-build-once-promote-by-sha.md)). Each store is written once and only read from within its own project.

## Consequences

A new per-environment store must exist in prod before a merge writes to it ([24](0024-manual-prod-infrastructure-apply.md)).
