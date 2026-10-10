# 1. Manifests live with their code

Status: Accepted

## Context

A manifest at the repository root claims the whole tree for one language. A root `go.mod` makes `go mod tidy` walk `infrastructure/` and the frontend, roots editor tooling's workspace at the repository, and outlives the code that justified it if that service is ever removed.

## Decision

Each language's manifest lives in the directory that owns that language's code: `services/backend/go.mod`, `services/frontend/package.json`, the Terraform roots under `infrastructure/`. `.github/dependabot.yml` is the same list read back: every `directory` names the manifest's own folder, and the one entry rooted at `/` is `github-actions`, which really is repository-wide.

Shared Go code, when there is any, gets a second module beside the first (`packages/go`, consumed through a `require` paired with a relative `replace`) rather than one module hoisted to the root.

`.moon/` and `.prototools` (#191) sit at the root anyway, as `pants.toml` once did. They configure the orchestrator that walks every other manifest rather than claiming an ecosystem of their own, so there is nothing for them to live beside.

## Consequences

- Each language's tooling only sees its own directory.
- A new shared Go package needs a second module and a `replace`, not a move to the root.
