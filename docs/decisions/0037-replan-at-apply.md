# 37. Applies plan again in the job

Status: Accepted (supersedes [32](0032-apply-saved-plans.md))

## Context

Applies used the binary plan a PR or pre-release had saved under `plans/<env>/` in the state bucket ([32](0032-apply-saved-plans.md)), so that what was applied was what was shown. That needs the PR's plan job to write a plan an apply will accept, and a plan holds every variable in plain text. Once a pull request's workflow runs as an identity that can read no secret ([38](0038-separate-plan-and-apply-identities.md)), its plan is made with placeholder tokens and `-refresh=false`: applying it would apply the placeholders.

## Decision

- `terraform-pull-request` posts a plan and saves nothing. `terraform-push-commit` plans and applies in one job, so what is applied is exactly what was planned from that commit, and shows the plan in the job summary. A plan with nothing to apply is skipped.
- The review is the plan comment on the PR (root, staging and prod) and on the release PR (prod, posted by `plan-prod`, [39](0039-releases-from-release-please.md)). A PR plan is the configuration against the last applied state, not a refresh, so a drift an apply would refresh away can appear only at apply time.
- The `plans/` lifecycle rules on the state buckets are removed.
- `apply-infrastructure-prod.yaml` plans on the spot as before ([24](0024-manual-prod-infrastructure-apply.md)).

## Consequences

The fallback to "plan and apply on the spot" is now the only path, so a stale or missing saved plan can no longer surprise a merge or a rollback. An apply can differ from the plan a reviewer saw if state or the provider's refresh changed in between; the job summary shows what it did.
