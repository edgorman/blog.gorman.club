# 40. Dependency updates come from self-hosted Renovate

Status: Accepted

## Context

Dependabot's version updates covered GitHub Actions, Terraform providers, npm and Go modules, but not the pins outside those formats: the Dockerfile's base images, `.prototools`, the Terraform version repeated across files, the tools CI installs with `go install` or `pip`, and wrangler. They drifted or were synced by hand, and a coupled bump (Go in `.prototools`, `go.mod` and the Dockerfile) arrived as a failing PR that needed manual follow-up.

## Decision

- Renovate runs from `renovate.yaml`, a scheduled (weekly) and manually dispatchable workflow in the repository itself, not the hosted Mend app. Its config is `.github/renovate.json5`, and `dependabot.yml` is removed; Dependabot alerts and security updates stay on.
- It runs with the `github_provider_token` PAT, so its PRs trigger the PR checks (one opened with `GITHUB_TOKEN` would not), as the `github-renovate` identity, which can read that one secret and nothing else, because the job runs a third-party action ([38](0038-separate-plan-and-apply-identities.md)).
- One grouped PR per area, so a run opens at most four: `backend-deps` (Go modules, Docker images, the Go toolchain), `frontend-deps` (npm in `services/frontend`), `infrastructure-deps` (Terraform providers and version) and `ci-deps` (GitHub Actions, `packages/api` tooling and the `# renovate:` pins). Labels are as before, titled `build(deps): ...` so the `pr-title` check passes. Docker base images are pinned by digest. A pin that lives in several files moves in one PR: Go (`.prototools`, `go.mod`, the Dockerfile) and the Terraform version.
- A hand-written pin Renovate would not otherwise see carries a `# renovate: datasource=... depName=...` comment on the line above, which one regex manager reads: `.prototools`, the tools installed in `security.yaml`, `oasdiff` in `packages/api/moon.yml`, wrangler, and the proto version in the session hook.

## Consequences

- The PAT needs Contents, Pull requests, Workflows, Issues and Commit statuses read/write, so one token now holds Contents and Workflows write. Only the deploy and renovate identities can read it.
- GitHub pauses a scheduled workflow after 60 days without repository activity; running Renovate by hand restarts it.
- The Firestore emulator jar's pin and checksum in `pull-request.yaml` stay manual.
