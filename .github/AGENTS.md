# .github

Workflows, composite actions, repository settings and the moon setup CI runs through. What to do when a PR's checks fail is the `steward` skill (`.claude/skills/steward/SKILL.md`). Reasoning for each rule is in the linked record under `docs/decisions/`.

## CI/CD, Branching, & Release Lifecycle

- Everything builds, tests and deploys through GitHub Actions; infrastructure applies before services update.
- Every pipeline has an environment-scoped concurrency group.
- Shared steps live in composite actions under `.github/actions/`.
- An apply plans and applies in the same job from its own commit; the PR comment is the review, and a plan is never saved or passed between jobs ([37](../docs/decisions/0037-replan-at-apply.md)).
- Each workflow authenticates as its own identity: PR checks as `PLAN_`, `push-commit.yaml` as `DEPLOY_`, promotion and the manual prod apply as `PROMOTE_`, Renovate as `RENOVATE_SERVICE_ACCOUNT`. A PR's plan job gets placeholder tokens and `-refresh=false`, never a secret; a workflow that needs another identity calls a reusable workflow ([38](../docs/decisions/0038-separate-plan-and-apply-identities.md)).
- Renovate (`renovate.yaml`, `renovate.json5`) opens the dependency PRs. A hand-written version pin gets a `# renovate: datasource=... depName=...` comment above it ([40](../docs/decisions/0040-self-hosted-renovate.md)).

- `security.yaml` is advisory: its scanners report in the job summary and never fail a run or gate a merge; a scanner that cannot run does fail ([35](../docs/decisions/0035-advisory-security-scanning.md)). Each scanner has a moon marker task (`scan-go`, `scan-image`, `scan`, `scan-workflows`, `scan-secrets`) and a `changes` job skips the ones a pull request doesn't reach; the weekly and manual runs scan everything ([41](../docs/decisions/0041-moon-gates-every-ci-job.md)).
- moon decides which jobs run: each workflow's `changed` job runs `moon query affected` (moon only, `auto-install: false`) and every other job gates on its outputs. A new job needs a marker task with `inputs` and an output in `changed`, or it runs on every change. `lint-check`, `test` and `api-drift` are separate jobs, not a matrix, so a skip keeps the required check name ([41](../docs/decisions/0041-moon-gates-every-ci-job.md)).
- `push-commit.yaml` diffs against the previous commit when its run succeeded, else the last release tag, so a merge builds only what it changed; the backend, worker and frontend deploy in parallel as the `services` matrix (the frontend's steps are the `frontend-staging` action) and `smoke` is skipped when nothing deployable changed ([41](../docs/decisions/0041-moon-gates-every-ci-job.md)).

### Building with moon

- Every folder under `services/` or `packages/` needs a `moon.yml`; `root:projects-covered` fails otherwise ([19](../docs/decisions/0019-hand-declared-moon-graph.md)).
- Declare `dependsOn` by hand. Affected detection is changed files only, so a task that must react to another project lists it in `inputs` (`project://...`) ([19](../docs/decisions/0019-hand-declared-moon-graph.md)).
- Keep `moon ci`'s targets split one-to-one with the required jobs (`lint-check`, `test`, `api-drift`; `pr-title` and `api-breaking` in `conventions.yaml`) ([19](../docs/decisions/0019-hand-declared-moon-graph.md)).
- `deploy-{root,staging,prod}:plan` (pull requests) and `deploy-{root,staging}:apply` (merges) are `noop` markers whose `inputs` decide which environments a diff reaches. List only the workflow and actions that marker's own job uses, so a PR plan ignores `push-commit.yaml` and a merge apply ignores `pull-request.yaml`, and never set `runInCI: false` on one ([20](../docs/decisions/0020-deploy-markers.md)).
- Pin tool versions only in `.prototools`; keep its `go` pin, `go.mod` and the Dockerfile image in step ([21](../docs/decisions/0021-prototools-pins-the-toolchain.md)).
- No remote cache ([18](../docs/decisions/0018-moon-replaced-pants.md)).

### Versioning

- release-please computes versions from Conventional Commit subjects and opens the release PR; tags are `vMAJOR.MINOR.PATCH` and `build:`, `chore:`, `ci:`, `docs:` and `refactor:` call for a patch release too, so dependency bumps reach prod; `style:` and `test:` alone do not ([39](../docs/decisions/0039-releases-from-release-please.md)).

### Staging Deployments

- Every merge publishes the backend and worker images and the frontend `dist/` by commit SHA to both staging's and prod's stores ([22](../docs/decisions/0022-build-once-promote-by-sha.md)). Only what its `publish` marker marks affected is built; the rest is the last release's, re-tagged or re-published, and an unchanged image isn't redeployed. List every action and workflow a service's build uses in its marker ([33](../docs/decisions/0033-build-only-what-changed.md)).
- The frontend bundle carries nothing environment-specific; `config.json` is written at deploy time ([22](../docs/decisions/0022-build-once-promote-by-sha.md)).

### Release PRs

- `release-please` runs on every merge and checks each direct need for `success` (`services`, a matrix over the backend, worker and frontend, and `smoke`, which may be skipped), never a bare `needs:` ([39](../docs/decisions/0039-releases-from-release-please.md)).
- `plan-prod` posts the prod plan on the open release PR, with `-lock=false` ([39](../docs/decisions/0039-releases-from-release-please.md)).
- Staging shows the short commit SHA; only production shows a release tag ([39](../docs/decisions/0039-releases-from-release-please.md)).

### Production Releases

- Merging the release PR is the promotion: `release-please` tags the commit and `promote-release.yaml` runs. It applies prod Terraform, then deploys the backend and the frontend by commit SHA from prod's own stores. Never rebuild for production, and never copy between projects ([22](../docs/decisions/0022-build-once-promote-by-sha.md)).
- The `release-<version>` image tag is for retention only; deploy by SHA ([29](../docs/decisions/0029-artifact-retention.md)).
- Rollback is running `promote-release.yaml` by hand with an earlier release tag ([22](../docs/decisions/0022-build-once-promote-by-sha.md)).
- `apply-infrastructure-prod.yaml` is only for backfilling a new per-environment resource a merge depends on; say so in that PR ([24](../docs/decisions/0024-manual-prod-infrastructure-apply.md)).
