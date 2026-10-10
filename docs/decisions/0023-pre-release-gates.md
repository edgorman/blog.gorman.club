# 23. What a pre-release waits for

Status: Accepted

## Context

Every merge to `main` cuts a GitHub pre-release with the commit summary and a production `terraform plan -lock=false` (read-only and unlocked, so it never contends with a real prod apply). A tag cut for a commit whose artifacts were never published would have nothing to deploy or roll back to.

## Decision

- The version is computed by its own `version` job, gated on nothing, since computing it is read-only. `services-frontend` writes that exact tag into staging's `config.json`, and `pre-release` takes the tag from `version`'s output rather than its own recompute, so the displayed and cut tags can't disagree.
- `pre-release` runs on every merge but needs `services-backend`, `services-worker`, `services-frontend`, `smoke` and `version`, with `if: always() && <each direct need> == 'success'`. `smoke` (#277) runs the Playwright suite in `services/frontend/e2e` against staging, so a merge that breaks staging end to end gets no release candidate.

History: #156 first used `always()` with `success || skipped`, which allowed exactly the case to stop, since neither publish job has a benign skip. Dropping `if:` instead made `pre-release` inherit a transitive skip from `infrastructure-root`/`-staging` (skipped on most merges), so it skipped nearly every merge while staging already advertised the next version (#185). Checking only direct needs fixes both.

## Consequences

A new job a release depends on goes in both `needs` and the `if:`.
