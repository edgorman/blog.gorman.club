# 20. Deploy decisions come from marker tasks

Status: Accepted

## Context

CI has to decide which Terraform environments a diff reaches, including through workflow and composite action files.

## Decision

- Each `deploy-{root,staging,prod}:plan` task (read by pull-request.yaml) and `deploy-{root,staging}:apply` task (read by push-commit.yaml) is a `noop` whose only job is its `inputs`: `terraform.tfbackend`, `terraform.tfvars`, `project://infra-root` or `project://infra-env`, and the workflow and composite action that job uses (`terraform-pull-request` and `pull-request.yaml` for a plan; `terraform-push-commit` and `push-commit.yaml` for an apply, plus `gcp-secret-manager` for root's apply, since a plan reads no secret). Plan and apply are separate so editing one workflow doesn't re-plan or re-apply what the other covers. There is no blanket `.github/**` glob, so an action only staging uses doesn't mark prod affected.
- The `changed` job runs `moon query affected` with `MOON_BASE` set to `origin/main` on a PR, and on a push to the newest `v*` tag before the pushed commit, so a merge whose run was cancelled while pending is still applied by the next. A marker in the affected set runs that environment's `terraform plan` (PR) or `apply` (push).
- Markers keep `options.cache: false` and the default `runInCI`. A `runInCI: false` task is invisible to `moon query affected` in CI, which once made every environment read unaffected.

## Consequences

A new workflow or action an environment depends on must be added to that marker's `inputs`.
