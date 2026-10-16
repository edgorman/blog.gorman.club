# 32. Applies use the plan already shown

Status: Accepted (amends [20](0020-deploy-markers.md) and [23](0023-pre-release-gates.md))

## Context

A PR comments its root and staging plans, and a pre-release's notes carry the prod plan, but every apply then planned again on the spot. What was reviewed and what was applied could differ.

## Decision

- `terraform-pull-request` takes a `plan_dir` and saves its binary plan there as `<git tree hash>.tfplan`. `pull-request.yaml` saves root and staging plans; `pre-release` saves the prod plan.
- `terraform-push-commit` takes the same `plan_dir` and applies the plan for its checkout's tree. Plans are keyed by tree, not commit: a PR's merge commit and its squash commit on `main` differ in SHA but, with branches required to be up to date, not in tree. A release commit has the tree its pre-release planned.
- Plans live in each environment's state bucket under `plans/<env>/`. A plan holds every variable in plain text (root's include the Cloudflare and GitHub tokens) and the repository is public, so workflow artifacts are out; the state bucket already holds the same values. Lifecycle rules delete plans after 90 days and replaced versions after 7.
- A missing plan (the PR didn't reach that environment, but the merge diff does after a cancelled run) or a stale one (Terraform refuses a plan whose state changed since, as on every rollback) falls back to planning and applying on the spot, with a warning and the plan in the job summary. A plan with no changes is not applied.
- `apply-infrastructure-prod.yaml` passes no `plan_dir` and always plans on the spot ([24](0024-manual-prod-infrastructure-apply.md)).

## Consequences

The fallback keeps merges and rollbacks from stalling, at the cost that an apply after a fallback was not reviewed beforehand; the warning and job summary say so.
