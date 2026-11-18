# 41. moon decides which CI jobs run

Status: Accepted (amends [20](0020-deploy-markers.md) and [33](0033-build-only-what-changed.md))

## Context

moon already chose what the Terraform plans and applies and the image and bundle builds reach, through `moon query affected`. The security scanners, the staging smoke test and the Firestore emulator used no such list, and the `lint-check`, `test` and `api-drift` jobs started a runner and a full toolchain even when `moon ci` had nothing to run. A release PR, which changes only generated files, still ran most of CI. A merge built everything changed since the last release, not only what that merge changed, and took 6 to 10 minutes.

## Decision

- Every job whose work depends on files has a moon task that lists those files as `inputs`: the existing `deploy-*:plan`, `deploy-*:apply` and `*:publish` markers, plus `scan-go`, `scan-image`, `scan`, `scan-workflows` and `scan-secrets` for the scanners. A marker is a `noop` task with `cache: false` and the default `runInCI`, as in [20](0020-deploy-markers.md).
- Each workflow's first job, `changed`, runs `moon query affected` once and outputs which jobs have work. Every other job gates on those outputs with `if:`. `changed` installs only moon (`auto-install: false` on `moonrepo/setup-toolchain`) and runs the query with `MOON_TOOLCHAIN_FORCE_GLOBALS`, `MOON_SKIP_SETUP_TOOLCHAIN` and `GOTOOLCHAIN=local`, so moon neither installs the pinned tools nor lets the runner's go download go.mod's toolchain; that took the job from about 40s to about 18s. Jobs that run tasks install the toolchain.
- The three `moon ci` legs are separate jobs named `lint-check`, `test` and `api-drift`, not a matrix. A job skipped by `if:` reports `skipped`, which branch protection counts as passing, and a skipped matrix leg would report an unexpanded name and stay pending. The targets each job runs are the ones `changed` checks. Branch protection requires one job, `required`, which waits for them all and fails only if one that ran failed, plus `pr-title` and `api-breaking`; naming the skippable jobs listed six skipped checks as required on a release PR.
- `api-breaking` installs moon, asks it whether `packages/api:breaking` is affected, and installs the toolchain only if so.
- A merge diffs against the last commit its workflow fully processed ([33](0033-build-only-what-changed.md), #262): the previous commit when its own `push-commit.yaml` run succeeded, otherwise the newest release tag. A cancelled or failed run is never a base, so its work is redone.
- `backend`, `worker` and `frontend` build only what the merge changed, and reuse the previous commit's artifacts for the rest. The backend and frontend deploy in parallel, and `smoke` is skipped when nothing deployable changed.
- Release-please's generated files (`CHANGELOG.md`, `.github/.release-please-manifest.json`, `.github/version.txt`) are inputs to no marker, so a release PR affects nothing and its jobs are skipped.
- Schedule and manual runs have no diff, so `security.yaml` runs every scanner on them.

## Consequences

Adding a job means adding its marker and its `changed` output, or it runs on every change. moon sees only files: a repository variable, an event type or a changed secret still needs an `if:` of its own. `changed` runs on every PR and merge and costs a few seconds.
