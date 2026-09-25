---
name: steward
description: This repo's conventions for driving a pull request to green - what each required check means, how to reproduce it locally, and which failures need the user. Use when watching, fixing CI on, or answering review on a PR in edgorman/blog.gorman.club.
---

# Stewarding a PR in this repo

Why things are set up this way lives in the `AGENTS.md` files; this is only what to do. Build and CI: `.github/AGENTS.md`. Wire contract: `packages/protos/AGENTS.md`.

## Before any push

Run what CI runs, from the repo root, against `origin/main` (`git fetch origin main` first):

```
moon ci --base origin/main :format :lint :vet :typecheck :build root:wire-types root:projects-covered root:go-version-sync
moon ci --base origin/main :test
moon ci --base origin/main packages/protos:drift
```

`moon ci` runs only what the diff affects, so a clean run on an untouched project proves nothing about it. In a cloud session two targets cannot run: `:validate` (`terraform init` needs `registry.terraform.io`) and `services/backend:image` (no Docker daemon). For those, read the CI log.

## Required checks

All seven jobs of `.github/workflows/pull-request.yaml` are required (`.github/settings.yml`). A job its `if:` skips reports "skipped", which counts as passing.

| Check | What a failure means | Fix |
|---|---|---|
| `lint-check` → `:format` | `gofmt`, `buf format` or `terraform fmt` | `gofmt -w`, `buf format -w` (from `packages/protos`), `terraform fmt -recursive infrastructure`. The PostToolUse hook already formats `.go`/`.proto` on edit |
| `lint-check` → `:lint`/`:vet`/`:typecheck`/`:build` | oxlint, `buf lint`, `go vet`, `tsc`, `vite build` | Fix the code |
| `lint-check` → `:validate` | `terraform validate` | Fix the HCL. You can't reproduce it locally, so read the log |
| `lint-check` → `root:wire-types` | An `export interface` in `services/frontend/src/lib/api.ts`, or a `json:"..."` tag in `services/backend/internal/service` outside `debug.go` | Use the generated type from `src/gen` / `internal/gen`. If the type doesn't exist yet, add the message to a `.proto` |
| `lint-check` → `root:projects-covered` | A `services/*` or `packages/*` folder has no `moon.yml` | Add one, modelled on a sibling |
| `lint-check` → `root:go-version-sync` | `.prototools`'s `go` pin and `services/backend/go.mod`'s `go` line differ | Change both. Also bump `FROM golang:X-alpine` in `services/backend/Dockerfile` (#123) |
| `test` → `:test` | `go test` or Jest | A real failure. Root-cause it |
| `test` → `services/backend:image` | The Docker build | Usually a Go version or a file missing from the build context |
| `protos-drift` | Generated code doesn't match the `.proto` | `cd packages/protos && buf generate`, then commit `services/backend/internal/gen` and `services/frontend/src/gen`. **Never hand-edit either folder** |
| `changed` | `moon query affected` failed | A `moon.yml` is invalid. `MOON_BASE=origin/main moon query affected` reproduces it |
| `infrastructure-{root,staging,prod}` | `terraform init/fmt/validate/plan` with real credentials | See below |

**Toolchain errors are not test failures.** `proto-shim: ... Permission denied (os error 13)` is the known toolchain cache bug that `cache: false` works around in every job. If it shows up anyway, it failed before any task ran, so one re-run is allowed.

## Infrastructure plans

The plan job posts its output as a PR comment. Read it:
- Check that every change in the plan is one the diff meant. Flag any `destroy` or `replace` to the user, even when CI is green.
- `fmt`/`validate` errors: fix them.
- Plan errors (permissions, API not enabled, provider errors, state): stop and comment on the PR with the error. These need the user.
- **Never** run `terraform apply`, `gcloud run deploy`, `wrangler` or anything else that mutates an environment. `.claude/settings.json` denies them anyway.
- A new per-environment resource that a merge-time job writes to also needs `apply-infrastructure-prod.yaml` run after merge. The user does that; say so in the PR body.

## Branch and merge rules

- **Title**: a Conventional Commit (`feat(backend): ...`, `fix(ci): ...`). PRs are squash-merged with the PR title as the commit subject, and versioning is computed from that subject. Fix a non-conforming title on a PR you opened.
- **Body**: follow `.github/PULL_REQUEST_TEMPLATE.md` (Summary, `Closes #`, Area checkboxes, Testing).
- **`strict: true`**: the branch must be up to date with `main`. When it's behind, merge `origin/main` in. Never rebase or force-push; `allow_force_pushes: false` and squash merge keep `main` linear anyway.
- **Approval**: one code-owner approval (`@edgorman`), and `dismiss_stale_reviews` is on, so any push drops it. Don't hold back a needed fix because of this. Once CI is green with no open threads, the PR is waiting on the owner and nothing else is yours to do.
- **`mergeable_state: blocked` on a green PR is expected.** Claude Code pushes and opens PRs as `@edgorman`, and GitHub never lets an author approve their own PR, so the owner merges with the admin override (`enforce_admins: false`). Don't investigate it, and never propose dropping `required_pull_request_reviews` (#210, reverted by #211).
- **`required_conversation_resolution`**: resolve every thread you've addressed, and reply on any you're not changing.

## Dependabot PRs

`build(deps)` PRs come grouped per ecosystem (`gomod-deps`, `npm-deps`, `terraform-deps`, `ci-deps`):
- Go: if the bump moves `go.mod`'s `go` line, `root:go-version-sync` fails. Bump `.prototools` and the `Dockerfile` image in the same PR.
- `google.golang.org/protobuf`: `packages/protos/buf.gen.yaml` pins `protoc-gen-go@<version>` to match `go.mod`. Bump it, run `buf generate`, and commit the output, or `protos-drift` fails.
- `ts-proto` (npm): same idea. Run `buf generate` and commit `src/gen`.
- Terraform providers: the plan comment is the review. Read it for unexpected changes.

## Never

- Hand-edit `internal/gen` or `src/gen`.
- Skip, disable or loosen a test or check to get green, including editing `root:wire-types`'s grep or a task's `inputs` to dodge a failure.
- Add a `CLAUDE.md`, `.claude/CLAUDE.md` or `CLAUDE.local.md`. Any one of them makes Claude Code stop loading every `AGENTS.md`.
- Put a manifest (`go.mod`, `package.json`, ...) at the repository root.
