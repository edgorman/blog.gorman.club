# 18. moon replaced Pants

Status: Accepted (supersedes the Pants setup from #110)

## Context

#110 adopted [Pants](https://www.pantsbuild.org/) to stop maintaining the dependency graph by hand. By #190 most of the cost had moved to fighting the tool:

- `go vet` ran outside Pants, because its vet backend never changed into the module directory and `services/backend/go.mod` isn't at the build root (#114).
- The frontend bundle was never built through Pants. `node_build_script` is a field value, not a target; called as a bare statement in `services/frontend/BUILD` from #119 on, it built nothing, so #146's "green in CI" and #156's first packaging step were vacuous. Registering it properly failed, because the sandbox held only declared dependencies and nothing imports a tsconfig, `index.html` or `public/` asset.
- oxlint had no Pants backend.
- The sandbox dropped `PATH`, breaking `terraform init` until patched.
- A Python interpreter was needed only to parse HCL.
- 17 BUILD files plus `pants tailor --check`, some with hand-declared edges anyway.
- Composite actions were invisible to the graph, so a blanket `.github/**` glob was ORed onto every infrastructure decision.
- Pants couldn't bootstrap in a Claude Code cloud session (its bundled Python rejected the proxy's CA).
- Three `tsconfig*.json` settings existed only for Pants.

## Decision

Replace Pants with moon v2 (#190-#200): a per-project graph of hand-declared edges plus per-task `inputs` ([19](0019-hand-declared-moon-graph.md)). The migration ran moon alongside Pants first (#198's advisory jobs, matching Pants across a soak period and a real merge, #212) and deleted nothing until parity held (#199, #200).

Deliberately given up:

- Per-file dependency inference. With one Go module, one npm package and no cross-project imports, there is nothing for it to find.
- A sandbox. moon runs in the real checkout, so undeclared inputs aren't caught; `inputs` must be explicit where it matters.
- The GitHub Actions remote cache. moon's remote cache speaks only Bazel Remote Execution v2. `moon ci` skipping unaffected work covers most of it; revisit (e.g. `bazel-remote` on GCS) only if CI time becomes a problem.

## Consequences

The frontend bundle builds as `services/frontend:build`, and the backend image as `services/backend:image`, a plain `inputs` glob replacing Pants' hand-declared list.
