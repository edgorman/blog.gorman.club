# Decision records

One file per decision: its context, the decision, and its consequences. The rules that follow from them are in the `AGENTS.md` files, each linking here. A record changes only when it is superseded: write a new one, and mark the old one `Superseded by N`.

| # | Decision | Area |
| --- | --- | --- |
| [1](0001-manifests-live-with-their-code.md) | Manifests live with their code | repo |
| [2](0002-generated-code-is-committed.md) | Generated code is committed, next to its consumer | protos |
| [3](0003-pin-every-codegen-tool.md) | Every codegen tool is pinned exactly | protos |
| [4](0004-ts-proto-options.md) | ts-proto options | protos |
| [5](0005-protos-model-the-wire.md) | Protos model the wire | protos |
| [6](0006-protojson-settings.md) | protojson settings | protos |
| [7](0007-visibility-is-a-string.md) | Visibility is a string, not an enum | protos |
| [8](0008-allowed-emojis-stay-literals.md) | Allowed emoji stay as literals | protos |
| [9](0009-no-hand-written-wire-types.md) | No hand-written wire types | protos |
| [10](0010-wire-changes-across-releases.md) | Wire changes across the release window | protos |
| [11](0011-assistant-on-agent-platform.md) | The assistant uses the Agent Platform, no API key | backend |
| [12](0012-assistant-entitlement-is-a-subscription.md) | The assistant is a subscription, rate limited | backend |
| [13](0013-one-access-policy-table.md) | One access policy table | backend |
| [14](0014-filters-narrow-the-feed.md) | Filters and search only narrow the feed | backend |
| [15](0015-cache-only-the-anonymous-feed.md) | Cache only the anonymous feed | backend |
| [16](0016-worker-is-a-second-binary.md) | The worker is a second binary | backend |
| [17](0017-comments-published-then-moderated.md) | Comments are moderated after publishing | backend |
| [18](0018-moon-replaced-pants.md) | moon replaced Pants | ci |
| [19](0019-hand-declared-moon-graph.md) | A hand-declared moon graph | ci |
| [20](0020-deploy-markers.md) | Deploy decisions come from marker tasks | ci |
| [21](0021-prototools-pins-the-toolchain.md) | `.prototools` pins the toolchain | ci |
| [22](0022-build-once-promote-by-sha.md) | Build once, promote by commit SHA | ci |
| [23](0023-pre-release-gates.md) | What a pre-release waits for | ci |
| [24](0024-manual-prod-infrastructure-apply.md) | A manual prod apply, for bootstrapping only | ci |
| [25](0025-versions-from-conventional-commits.md) | Versions come from Conventional Commits | ci |
| [26](0026-one-gcp-project-per-environment.md) | One GCP project per environment | infrastructure |
| [27](0027-workload-identity-federation.md) | Workload Identity Federation | infrastructure |
| [28](0028-per-environment-artifact-stores.md) | Per-environment artifact stores | infrastructure |
| [29](0029-artifact-retention.md) | Artifact retention | infrastructure |
| [30](0030-monitoring-on-cloud-run-metrics.md) | Monitoring on Cloud Run metrics | infrastructure |
| [31](0031-debug-endpoint-contract.md) | The debug endpoint contract | backend |
