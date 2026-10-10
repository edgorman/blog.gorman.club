# 19. A hand-declared moon graph, affected by changed files only

Status: Accepted

## Context

At about 8 projects, with one Go module and one npm package, inference has nothing to find that a few declared edges don't say more plainly ([18](0018-moon-replaced-pants.md)).

## Decision

- `.moon/workspace.yml` discovers `services/*` and `packages/*` by folder (`globFormat: 'source-path'`, so IDs are paths) and names `root`, `worker`, `infra-root`, `infra-env` and `deploy-{root,staging,prod}` explicitly. Each `moon.yml` declares `dependsOn` by hand.
- `root:projects-covered` fails CI if a `services/*` or `packages/*` folder has no `moon.yml`.
- `--affected` is changed files only; nothing uses `--include-relations`. A task that must react to another project lists it in `inputs` (`project://packages/protos` on both services' `test`). `.moon/tasks/all.yml`'s `implicitInputs` adds each project's own `moon.yml`, so editing a task runs it.
- `moon ci`'s targets map one-to-one to required checks, as legs of `pull-request.yaml`'s one `moon` matrix job, each named after its check: `lint-check` runs `:format :lint :vet :typecheck :build :validate root:wire-types root:projects-covered root:go-version-sync`, `test` runs `:test services/backend:image worker:image`, `protos-drift` runs `packages/protos:drift`, each against `origin/main`. `conventions.yaml` holds `pr-title` and `protos-breaking`, so a title edit reruns only those. A leading `:` runs the task in every project that has it.
- `services/backend:image` is built (not published) on every PR it reaches, so a broken image fails that PR, not the merge job. It has `cache: false` (its output is a Docker layer moon can't restore) and `runInCI: true`.
- Terraform roots are `infra-env`/`infra-root` with `format` and `validate` (`runInCI: 'only'`) from `.moon/tasks/terraform.yml`. moon runs unsandboxed, so `terraform init` sees the runner's `PATH`.

## Consequences

More typing per task than a transitive flag, and exactly as precise as the `inputs` declared.
