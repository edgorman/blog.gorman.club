# 24. A manual prod apply, for bootstrapping only

Status: Accepted

## Context

Staging applies on every merge, prod only on promotion, so prod state can lag `main`. A merge-time job that writes into every environment's store ([28](0028-per-environment-artifact-stores.md)) fails if the store is new and prod hasn't been promoted past the PR that added it.

## Decision

`apply-infrastructure-prod.yaml` (**Apply Prod Infrastructure (Manual)**, `workflow_dispatch`) runs only `terraform apply` on prod, with no release. It requires typing `apply` and shares Promote Release's concurrency group.

## Consequences

It is not the normal path. Prod infrastructure changes through a promotion, whose plan was visible in the pre-release notes. Use this only to backfill a new per-environment resource a merge is about to depend on, and say so in that PR.
