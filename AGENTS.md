# AGENTS.md

The repo-wide rules and conventions for this repository, for Claude Code, other coding agents and contributors. Each area keeps its own detail in an `AGENTS.md` of its own, loaded when working on files under that directory:

- `packages/protos/AGENTS.md` - the contract layer: buf, codegen, protojson settings, ts-proto flags, what is and is not a proto.
- `services/backend/AGENTS.md` - the AI writing assistant, access control, reader engagement, finding posts, caching, the container image.
- `infrastructure/AGENTS.md` - GCP projects, the root environment, IAM/WIF, resource naming, artifact stores and their retention, monitoring.
- `.github/AGENTS.md` - building with moon, CI/CD, versioning, staging deploys, pre-releases, production releases, rollback.
- `docs/architecture.md` - history and reasoning behind past decisions, rather than rules to follow.

Claude Code reads these through its built-in [`agents-md`](https://github.com/anthropics/claude-code/tree/main/mods/agents-md) plugin, whose default ("Project instructions" in `/config`: `claude-md-or-agents-md`) loads `AGENTS.md` exactly where it would load `CLAUDE.md` - but only while the project has no `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` of its own. Set to `claude-md`, or with the plugin turned off in `/plugin`, Claude Code loads none of them.

## Overview

This is a single-repository (monorepo), multi-cloud deployment strategy built on Trunk-Based Development. It manages a static frontend, containerized backend services, infrastructure code, and repository governance from one central location.

## Repository Structure

- `/infrastructure` — Centralized Terraform manifests. The `env` subfolder holds the manifests applied once per environment (staging/prod) using environment-specific variable configurations (`staging.tfvars`, `prod.tfvars`); the `root` subfolder holds shared, manually-bootstrapped resources (see Root Environment in `infrastructure/AGENTS.md`).
- `/services/backend` — Golang backend service(s) packaged as Docker containers targeted for GCP Cloud Run.
- `/services/frontend` — Conventional Vite/React single-page app deployed to Cloudflare Pages.
- Neither service carries its own `lint`/`test`/`build` Makefile: CI runs those through moon (see Building with moon in `.github/AGENTS.md`), and local development calls the language's own tooling directly (`go test`, `npm test`, ...) rather than through a wrapper.
- `/.github/actions` — Modular, local GitHub Composite Actions (`action.yml`) encapsulating reusable workflow logic.
- `/.github/settings.yml` — Repository settings, branch permissions, and rulesets managed declaratively as code via the Probot Settings App.
- `/.github/workflows` — Event-specific workflow YAMLs (e.g. pull request, commit, release) that use reusable GitHub Actions.

Each language's manifest lives in the directory that owns that language's code, and the repository root declares none of its own; shared Go code, when there is any, gets a second module (`packages/go`) rather than one hoisted to the root. See `docs/architecture.md` for why.


## Commands

What a contributor (or Claude Code) runs locally to check a change before pushing - each language's own tooling, per Repository Structure above, mirroring what `pull-request.yaml` runs in CI:

```
# Backend (services/backend)
go test ./... && go vet ./... && gofmt -l .

# Frontend (services/frontend)
npm test && npm run lint && npm run build

# Protos (packages/protos) - commit the regenerated internal/gen and src/gen alongside the .proto
buf lint && buf format -d --exit-code && buf generate

# Infrastructure
terraform fmt -check -recursive infrastructure

# CI parity (repo root) - what pull-request.yaml runs
moon ci --base origin/main                               # everything affected vs origin/main
moon run services/backend:test services/frontend:lint    # specific targets
MOON_BASE=origin/main moon query affected                # what CI's `changed` job considers affected (no --base flag; bare, it only sees uncommitted changes outside CI)
```

Never run `terraform apply`, `gcloud run deploy`, `wrangler` or anything else that mutates a real environment by hand: every one of those goes through the workflows described in `.github/AGENTS.md`. `.claude/settings.json` denies them to Claude Code outright, and no moon task wraps them either.

In a Claude Code cloud session, `.claude/hooks/session-start.sh` installs the pinned toolchain these need, including the `moon`/`proto` binaries themselves (pinned GitHub release downloads, the same pattern buf/terraform use). moon fetches its plugins from `ghcr.io` and proto fetches Go from `dl.google.com`, neither of which is in the default allowlist, so the hook also exports `MOON_PLUGINS_USE_URL_DIST` (plugins from their GitHub release assets instead) and `MOON_TOOLCHAIN_FORCE_GLOBALS` (the tools already on `PATH` instead of proto-installed copies). With those, `moon run` and `moon ci` work in a cloud session, except for two tasks: `infra-root:validate`/`infra-env:validate`, since `terraform init` needs `registry.terraform.io`, and `services/backend:image`, since the session has no Docker daemon. `moon setup`/`proto install` still don't work there, and don't need to. Local macOS/Linux sessions aren't behind this proxy: `proto install && moon ci` is the whole setup there. The Go, npm and buf commands work with the default allowlist regardless.

`.claude/settings.json` also enables the team's Claude Code plugins (`enabledPlugins`, from the marketplaces in `extraKnownMarketplaces`). Locally, Claude Code offers to install them once you trust the folder. A cloud session never installs a repository's plugins itself, so `.claude/hooks/install-plugins.sh` does it, reading the same two keys, in the background at session start. The session has already loaded its plugins by the time the hook runs, so they become active on the next start or resume, or straight away with `/reload-plugins`. To have them from the first message, enable them for your claude.ai account instead, which cloud sessions load as synced plugins.

## Rules

Short versions; each links to where the full reasoning lives.

- **Never run apply/deploy by hand.** `terraform apply`, `gcloud run deploy`, `wrangler` and the like go through the workflows (`.github/AGENTS.md`).
- **Never hand-edit `services/backend/internal/gen` or `services/frontend/src/gen`.** Change the `.proto`, run `buf generate`, and commit the result with it; CI fails on drift (`packages/protos/AGENTS.md`).
- **Protos model the wire, not the domain or the datastore.** Validation and policy stay in `internal/entity`, storage shapes in `internal/repository/firestore` (`packages/protos/AGENTS.md`).
- **No hand-written wire types.** No `export interface` in `api.ts`, no `json:"..."` tags in `internal/service` outside `debug.go`; CI enforces both (`packages/protos/AGENTS.md`).
- **Every new resource or action needs a line in the access policy table** (`services/backend/internal/entity/access.go`); an undeclared pair is refused (`services/backend/AGENTS.md`).
- **Images and bundles are never rebuilt for production.** Prod deploys the commit-SHA artifact already in its own store (`.github/AGENTS.md`).
- **Never add a manifest at the repository root** (Repository Structure above).
- **Never add a `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` anywhere in the repository.** One such file makes Claude Code ignore every `AGENTS.md` here (see above); put instructions in the `AGENTS.md` of the directory they apply to.

## Commit Message Convention

Every PR title must be a valid [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) subject — `type(scope)?: description`, e.g. `feat(backend): paginate GET /blogs` — because the repo squash-merges every PR (`.github/settings.yml` sets `squash_merge_commit_title: PR_TITLE`), so the PR title becomes the subject line of the one commit that lands on `main`, and that's what Versioning (in `.github/AGENTS.md`) actually parses. Use `feat:` for a new capability, `fix:` for a bug fix, and any other Conventional Commits type (`build`, `chore`, `ci`, `docs`, `perf`, `refactor`, `revert`, `style`, `test`) for a change that shouldn't move the version on its own. Mark a breaking change with `!` after the type/scope (`feat!:`) or a `BREAKING CHANGE:` footer in the PR body — either bumps major regardless of type. A title that doesn't match any recognized type still merges fine; it just falls back to a patch bump, same as everything did before this convention existed. This applies to Claude Code equally: title PRs it opens the same way.
