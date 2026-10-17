# 34. Artifact stores are created by root, not by each environment

Status: Accepted (supersedes where [28](0028-per-environment-artifact-stores.md) put the stores; amends [24](0024-manual-prod-infrastructure-apply.md))

## Context

Every merge publishes the backend and worker images and the frontend bundle into every environment's stores ([22](0022-build-once-promote-by-sha.md), [28](0028-per-environment-artifact-stores.md)), but env Terraform only reaches prod on promotion. With the stores in env Terraform, the first merge after building the environments from nothing ([`bootstrap.md`](../bootstrap.md)) failed pushing to prod's registry, which didn't exist until the first promotion, and so did any merge after a new per-environment store was added. The way round it was the manual prod apply ([24](0024-manual-prod-infrastructure-apply.md)). The template made from this repository hit the same failure on its first merge.

## Decision

- `infrastructure/root/artifact_stores.tf` creates each environment project's Artifact Registry `backend` repository, its `<project id>-frontend` bucket, the CI service account's writer grants on both, and the Artifact Registry API, for every project in `gcp_projects`. They are still one store per environment, written once and read only within its own project ([28](0028-per-environment-artifact-stores.md)); only who creates them moved. The retention rules ([29](0029-artifact-retention.md)) and their variables moved with them.
- Root applies before staging on every merge it is affected by, and at bootstrap, so every environment's stores exist before anything publishes into them. `services-backend` and `services-frontend` both wait on root's apply having succeeded or skipped.
- The rule: anything a merge writes into every environment belongs in root. Env Terraform keeps what only its own environment's apply, deploy or promotion touches (Cloud Run, Firestore, monitoring).

## Consequences

A merge never depends on prod having been promoted, so the manual prod apply ([24](0024-manual-prod-infrastructure-apply.md)) is left for per-environment resources that aren't stores. A change to retention shows in root's plan, not the environments'.
