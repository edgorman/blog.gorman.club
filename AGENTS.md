# AGENTS.md

Repo-wide rules for Claude Code, other coding agents and contributors. Each area has its own `AGENTS.md`, loaded when working on files under it:

- `packages/protos/AGENTS.md` - the wire contract: buf, codegen, protojson, ts-proto.
- `services/backend/AGENTS.md` - the assistant, access control, engagement, search, caching, the worker.
- `infrastructure/AGENTS.md` - GCP projects, IAM/WIF, artifact stores and retention, monitoring.
- `.github/AGENTS.md` - moon, CI/CD, versioning, deploys, releases, rollback.

Rules are short; the reasoning behind each is a decision record in [`docs/decisions/`](docs/decisions/README.md). New to the repository? Start with [`docs/start-here.md`](docs/start-here.md).

Claude Code reads these files through its built-in [`agents-md`](https://github.com/anthropics/claude-code/tree/main/mods/agents-md) plugin, which loads `AGENTS.md` where it would load `CLAUDE.md`, but only while the project has no `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md`.

## Repository Structure

A trunk-based monorepo: a static frontend, containerized backend services, Terraform and repository governance in one place.

- `infrastructure/` - Terraform. `env` is applied per environment with `config/staging` and `config/prod`; `root` holds shared, bootstrapped resources.
- `services/backend/` - Go API for Cloud Run. `cmd/worker` is a second binary in the same module, its own moon project (`worker`), image and service.
- `services/frontend/` - Vite/React app on Cloudflare Pages.
- `packages/protos/` - the `.proto` contract both services generate from.
- `.github/` - workflows, composite actions (`actions/`), and `settings.yml` (repository settings and rulesets, applied by the Probot Settings app).
- `docs/` - [`start-here.md`](docs/start-here.md) and the decision records.

Each language's manifest lives in the directory that owns its code; the root declares none. Shared Go code gets a second module (`packages/go`) ([1](docs/decisions/0001-manifests-live-with-their-code.md)). Neither service has a Makefile: CI runs through moon, and locally you call each language's own tooling.

## Commands

What to run before pushing, mirroring `pull-request.yaml`:

```
# Backend (services/backend)
go test ./... && go vet ./... && gofmt -l .

# Frontend (services/frontend)
npm test && npm run lint && npm run build

# Protos (packages/protos) - commit the regenerated internal/gen and src/gen alongside the .proto
buf lint && buf format -d --exit-code && buf generate

# Infrastructure
terraform fmt -check -recursive infrastructure

# CI parity (repo root)
moon ci --base origin/main                               # everything affected vs origin/main
moon run services/backend:test services/frontend:lint    # specific targets
MOON_BASE=origin/main moon query affected                # what CI's `changed` job sees (no --base flag)
```

Locally, `proto install && moon ci --base origin/main` is the whole setup. In a Claude Code cloud session, `.claude/hooks/session-start.sh` installs the pinned toolchain and exports `MOON_PLUGINS_USE_URL_DIST` and `MOON_TOOLCHAIN_FORCE_GLOBALS`, since `ghcr.io` and `dl.google.com` aren't allowlisted. Everything then runs except `infra-root:validate`/`infra-env:validate` (`terraform init` needs `registry.terraform.io`) and `services/backend:image` (no Docker daemon); `moon setup` and `proto install` don't work there and aren't needed.

## Claude Code plugins

`.claude/settings.json` enables these plugins for every Claude Code session here, so expect their defaults in Claude-authored changes:

- `ponytail` - a minimal-code style: the smallest change that works, no speculative abstractions.
- `code-simplifier` - a cleanup pass over changed code.
- `feature-dev` - guided feature development agents (explore, architect, review).
- `google-cloud-developer` - Google Cloud docs lookup and `gcloud` guardrails.

Locally Claude Code offers to install them when you trust the folder. A cloud session installs them through `.claude/hooks/install-plugins.sh` in the background, so they apply from the next start, resume or `/reload-plugins`.

Two project skills in `.claude/skills/` cover issue to merged PR: `implement-issue` (e.g. `/implement-issue 194`) and `steward` (drive the PR to green).

## Rules

- **Never run apply/deploy by hand.** `terraform apply`, `gcloud run deploy`, `wrangler` and the like go through the workflows (`.github/AGENTS.md`); `.claude/settings.json` denies them to Claude Code.
- **Never hand-edit `services/backend/internal/gen` or `services/frontend/src/gen`.** Change the `.proto`, run `buf generate`, commit both (`packages/protos/AGENTS.md`).
- **Protos model the wire, not the domain or the datastore** (`packages/protos/AGENTS.md`).
- **No hand-written wire types.** No `export interface` in `api.ts`, no `json:"..."` tags in `internal/service` outside `debug.go`; CI enforces both (`packages/protos/AGENTS.md`).
- **Every new resource or action needs a line in the access policy table** (`services/backend/internal/entity/access.go`); an undeclared pair is refused (`services/backend/AGENTS.md`).
- **Images and bundles are never rebuilt for production.** Prod deploys the commit-SHA artifact already in its own store (`.github/AGENTS.md`).
- **Never add a manifest at the repository root** (Repository Structure above).
- **Never add a `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md` anywhere.** One such file makes Claude Code ignore every `AGENTS.md`; put instructions in the `AGENTS.md` of the directory they apply to.
- **Reasoning goes in a decision record, not an `AGENTS.md`.** Add a rule as a bullet linking to its record; supersede a record with a new one rather than rewriting it ([`docs/decisions/`](docs/decisions/README.md)).

## Commit Message Convention

Every PR title must be a [Conventional Commit](https://www.conventionalcommits.org/en/v1.0.0/) subject, `type(scope)?: description` (e.g. `feat(backend): paginate GET /blogs`), and the `pr-title` check enforces it. PRs are squash-merged with the title as the commit subject, and versioning parses that subject ([25](docs/decisions/0025-versions-from-conventional-commits.md)):

- `feat:` - a new capability; bumps minor.
- `fix:` - a bug fix; bumps patch.
- `build`, `chore`, `ci`, `docs`, `perf`, `refactor`, `revert`, `style`, `test` - bump patch.
- `!` after the type or scope, or a `BREAKING CHANGE:` footer in the PR body - bumps major.

This applies to PRs Claude Code opens too.
