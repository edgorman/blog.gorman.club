# blog.gorman.club

[![Push Commit](https://github.com/edgorman/blog.gorman.club/actions/workflows/push-commit.yaml/badge.svg?branch=main)](https://github.com/edgorman/blog.gorman.club/actions/workflows/push-commit.yaml)
[![Promote Release](https://github.com/edgorman/blog.gorman.club/actions/workflows/promote-release.yaml/badge.svg)](https://github.com/edgorman/blog.gorman.club/actions/workflows/promote-release.yaml)
[![Release](https://img.shields.io/github/v/release/edgorman/blog.gorman.club?include_prereleases)](https://github.com/edgorman/blog.gorman.club/releases)

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](services/backend)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](services/frontend)
[![Vite](https://img.shields.io/badge/Vite-646CFF?logo=vite&logoColor=white)](services/frontend)
[![Protobuf](https://img.shields.io/badge/Protobuf-buf_1.47-4285F4?logo=google&logoColor=white)](packages/protos)
[![Terraform](https://img.shields.io/badge/Terraform-1.15-844FBA?logo=terraform&logoColor=white)](infrastructure)
[![Cloud Run](https://img.shields.io/badge/GCP-Cloud_Run-4285F4?logo=googlecloud&logoColor=white)](infrastructure)
[![Cloudflare Pages](https://img.shields.io/badge/Cloudflare-Pages-F38020?logo=cloudflare&logoColor=white)](infrastructure)
[![moon](https://img.shields.io/badge/moon-2.5-6F53F3)](https://moonrepo.dev)

A demo of a small, cloud-deployed full-stack web service - a blog with a Gemini-backed writing assistant - built to show a trunk-based monorepo with CI/CD and infrastructure-as-code, rather than a real blog hosting platform.

## What's in here

| Path | What |
| --- | --- |
| `services/backend` | Go API (Firestore, Gemini), shipped as a container to GCP Cloud Run |
| `services/frontend` | Vite/React single-page app, deployed to Cloudflare Pages |
| `packages/protos` | Protobuf API contract; `buf generate` writes the Go and TypeScript types |
| `infrastructure` | Terraform for the shared root, staging and prod environments (GCP, Cloudflare, GitHub) |
| `.github` | Workflows and composite actions: PR checks, staging deploy on merge, release promotion to prod |
| `docs/architecture.md` | Why things are the way they are |

Every merge to `main` deploys to staging; production releases are promoted from there. Nothing is ever applied or deployed by hand.

## Developing with moon

[moon](https://moonrepo.dev) runs every task, and [proto](https://moonrepo.dev/proto) pins the toolchain (`.prototools`: Go, Node, npm, buf, Terraform, moon itself).

```sh
proto install                                   # install the pinned toolchain

moon run services/frontend:dev                  # frontend dev server
moon run services/backend:dev                   # backend on :8080 - needs GCP credentials for Firestore

moon run services/backend:test                  # one task in one project
moon run :lint                                  # one task across every project
moon ci --base origin/main                      # everything your branch affects - what CI runs on a PR

moon query projects                             # list projects
moon project services/backend                   # list a project's tasks
```

Each project's tasks live in its `moon.yml` (e.g. `services/backend/moon.yml`). If you change a `.proto`, regenerate with `buf generate` in `packages/protos` and commit the output alongside it.

PR titles must be [Conventional Commits](https://www.conventionalcommits.org/) (`feat(backend): ...`) since PRs are squash-merged and the title drives versioning. See [`AGENTS.md`](AGENTS.md) for the full contributor rules.
