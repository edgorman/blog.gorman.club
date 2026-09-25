# Architecture notes

History and reasoning behind past decisions: how the repository got to where it is, rather than rules to follow. The rules themselves live in `AGENTS.md` at the root and in each directory's own `AGENTS.md`.

## Manifests per directory

Each language's manifest lives in the directory that owns that language's code, and the repository root declares no ecosystem of its own: `services/backend/go.mod` is the Go module, `services/frontend/package.json` the npm project, `infrastructure/*/` the Terraform roots. `.github/dependabot.yml` is that same list read back — every `directory` there names the manifest's own folder, and the single entry rooted at `/` is `github-actions`, which genuinely is repository-wide. A manifest at the root would claim the whole tree for one language: a root `go.mod` makes `go mod tidy` walk `infrastructure/` and the frontend, roots editor tooling's workspace at the repository, and would outlive the code that justified it if the Go service were ever removed.

Shared Go code, when there is any, therefore gets a second module beside the first — `packages/go`, consumed through a `require` paired with a relative `replace` — rather than one module hoisted to the root to cover both. Generated code follows its consumer rather than the contract it came from: a shared `.proto` is the shared thing, while the Go and TypeScript bindings generated from it are build output committed for the sake of editor tooling, and belong next to the service that compiles them.

`.moon/` and `.prototools` (#191) sit at the repository root despite that rule, the same way `pants.toml`, `pants.ci.toml` and `get-pants.sh` used to (see "Build orchestrator history" below): none of them claims an ecosystem the way `go.mod` or `package.json` would. They configure the orchestrator that walks every other manifest, not a language of their own, so there's nothing for them to live "beside" — the rule against a root manifest is about not letting one language's tooling treat the whole tree as its own project, and an orchestrator doing that on purpose, for every language at once, is the point of having one.

## Build orchestrator history

### Why moon replaced Pants

#110 adopted [Pants](https://www.pantsbuild.org/) to stop maintaining the repo's dependency graph by hand. It achieved that, but by #190 most of the ongoing cost had shifted from maintaining the graph to fighting the tool that built it:

- **`go vet` had to run outside Pants.** `pants.backend.experimental.go.lint.vet` never changed into the module's own directory, so it only worked when `go.mod` sat at the Pants build root - which `services/backend/go.mod` deliberately doesn't (#114).
- **The frontend bundle was never built through Pants at all** (see below) - `npm run build` ran directly in CI instead, next to Pants rather than through it.
- **oxlint had no Pants backend**, so it too ran outside Pants.
- **The sandbox broke Terraform.** It dropped `PATH` entirely, which broke `terraform init` (`getent` not found) until `[download-terraform] extra_env_vars = ["PATH"]` patched it back in.
- **A Python interpreter was required just to parse HCL** (`[python] interpreter_constraints`), in a repository with no Python code of its own - Pants' Terraform backend infers dependencies by parsing `.tf` files with a Python-implemented HCL2 parser it runs as a PEX.
- **17 BUILD files, plus `pants tailor --check`** to catch a folder tailor's own globs didn't cover, some carrying hand-declared edges inference couldn't see on its own (`internal/gen` → `packages/protos`, the container image → `cmd/backend`, Jest config files).
- **A composite action was invisible to the graph.** Pants had no target type for one, so `github-file-diff` (`dorny/paths-filter`) survived as a blanket `.github/**` glob ORed onto every infrastructure decision, alongside the Pants-derived one.
- **Pants could not bootstrap in a Claude Code cloud session**, because its bundled Python rejected the session proxy's CA certificate.
- **Three `tsconfig*.json` settings existed only to satisfy Pants' TypeScript check**, not `tsc -b` itself.

At this repo's size (about 8 projects), tracking dependencies per file was not paying for itself. moon (#190-#200) replaced it with a per-project graph of hand-declared edges, plus per-task `inputs` - covering exactly what CI needs: lint/test what a diff affects, decide which Terraform deployment a diff reaches, and build the backend image on any PR that reaches it. The migration followed the same pattern #110 itself set: small PRs, moon running alongside Pants before replacing it (#198's advisory jobs, proven to match Pants' own answer across a full soak period and the real merge that landed it, #212), and nothing deleted until that parity held (#199, then #200).

**What moon deliberately does not replace, and why:**

- **Per-file dependency inference.** Pants resolved edges from imports; moon's edges in this repo are declared by hand in each `moon.yml`. With one Go module, one npm package, and no cross-project imports for either to infer from, there was nothing for inference to find that a handful of declared edges don't already say more plainly - the trade only costs something at a scale this repository isn't at.
- **A sandbox.** Pants ran every task in an isolated sandbox assembled from declared dependencies alone, which is what caused the frontend-bundle and Terraform `PATH` problems above but also caught an undeclared input the moment it was touched. moon runs tasks in the real checkout with the host environment; nothing here builds that safety net back, so `inputs` has to be explicit where it matters, on trust rather than enforcement.
- **The GitHub Actions remote cache.** Pants read and wrote it (`pants.ci.toml`, `experimental-github-actions-cache`), making a second CI run against an unchanged tree fast. moon's remote cache only speaks the Bazel Remote Execution v2 API, with no GHA backend - this repo accepted losing it, leaning on `moon ci` already skipping unaffected work instead. Revisit only if CI time becomes a real problem.

### Why the frontend bundle was not built through Pants

**Superseded by the move to moon above** - moon builds the frontend bundle directly (`services/frontend:build`) since it has no sandbox to hide a build input from. Kept as history, because the BUILD file looked correct for a long time while doing nothing. `node_build_script` is a **field value, not a target** - which is why it rejects `name=`, the symptom #119 hit and worked around by dropping the argument - so it belongs in `package_json`'s `scripts=` list. From #119 onward `services/frontend/BUILD` called it as a bare top-level statement, which constructs the object and discards it. The vite build was therefore never in the target graph: `pants package services/frontend::` exited in 190ms having matched nothing, which made both #146's "green in CI" and #156's first attempt at a packaging step vacuous passes rather than working builds.

Registering it properly (`package_json(scripts=[node_build_script(...)])`) makes the build real and immediately fails, because the sandbox a build script runs in is the `package_json` target's declared dependencies plus the npm install, not the directory tree - and nothing `import`s a tsconfig, `index.html` or a `public/` asset, so dependency inference reaches none of them: `error TS6053: File '.../services/frontend/tsconfig.json' not found`. Behind that sit `tsconfig.app.json`, `tsconfig.node.json`, `index.html`, every `public/` asset and most likely every `src/` target, all of which would have to be hand-declared - re-creating, in one `dependencies=` list, the per-file enumeration #155 removed from eight BUILD files. That trade isn't worth it for two services, so the `node_build_script` was left off deliberately and `npm run build` ran directly instead.

The backend image was the opposite call and worth contrasting: its hand-declared dependency list was three entries and bought a `docker build` that CI already depended on, so `pants package` owned it - moon's `services/backend:image` task (`services/backend/AGENTS.md`'s "Container image") plays the same role now, with a plain `inputs` glob in place of that hand-declared list.

## How `pre-release` came to check only its direct needs

`#156` first wrote it as `if: always()` with a `success || skipped` check on each, which permitted the very case it was meant to stop - neither publish job has a benign skip path, since each skips only when `changed` or an infrastructure job failed (a legitimately *skipped* infrastructure job leaves them running, because their own `if` accepts `skipped` there), so every skip is an upstream failure and none of them should produce a tag. The fix for that dropped `if:` entirely, reasoning that GitHub's default for a `needs` job - run only when every dependency succeeded, skip when one failed *or was itself skipped* - already gave the rule wanted here. That reasoning missed that GitHub's default success check isn't scoped to a job's own direct needs: it's also defeated by a `skipped` conclusion further back in the needs graph, including through a job like `services-frontend` that already overrides its own `if:` to tolerate exactly that skip from `infrastructure-root`/`infrastructure-staging` (which skip on almost every merge - most commits touch neither `infrastructure/**` nor `.github/**`). With a bare `needs:`, `pre-release` inherited that transitive skip and itself skipped on nearly every merge, even though its own direct needs had all succeeded - while `services-frontend` had already written that merge's speculative next version into staging's `config.json`, advertising a release that was never cut (#185). The fix keeps the `always()` `#156` already had - needed to opt back out of that same default short-circuit - but checks only `pre-release`'s own three direct needs (`needs.services-backend.result == 'success' && needs.services-frontend.result == 'success' && needs.version.result == 'success'`), stopping short of `#156`'s `success || skipped` and so keeping its fix: neither publish job has a benign skip path, so a skip of either is still not tag-worthy. `version` needs no carve-out in that condition either way, since it has no skip path of its own - it runs unconditionally.

## Health Verification Strategy

Before building application features, setup is validated using a lightweight Debug Endpoint Contract:

- **Backend** — Exposes a `/health` or `/debug` endpoint returning system status, timestamp, environment metadata, and git commit SHA.
- **Frontend** — A dashboard fetches the backend `/debug` endpoint on page load.
- **Outcome** — Green indicators on `staging.example.com` and `example.com` verify that cross-cloud DNS routing, CORS policies, environment variables, and project-isolated IAM bindings are fully operational.
