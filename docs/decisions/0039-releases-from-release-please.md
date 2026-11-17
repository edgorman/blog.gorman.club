# 39. Releases come from release-please

Status: Accepted (supersedes [23](0023-pre-release-gates.md) and [25](0025-versions-from-conventional-commits.md); amends [22](0022-build-once-promote-by-sha.md) and [24](0024-manual-prod-infrastructure-apply.md))

## Context

Every merge cut a GitHub pre-release, and production was deployed by editing one into a full release. That meant a version computed by a small pre-1.0 action, run twice and read back through an `awk` parse of its log; a releases page full of versions that never reached production; and a promotion that was a UI edit with no review and no view of what was about to ship.

## Decision

[release-please](https://github.com/googleapis/release-please) owns versions and releases, and merging its release PR is the promotion to production.

- **Every merge to `main`** applies root and staging infrastructure, builds and publishes both services by commit SHA to both stores ([22](0022-build-once-promote-by-sha.md), [33](0033-build-only-what-changed.md)), deploys staging and runs the smoke test. Then `release-please` (which waits on all of those, `if: always() && <each direct need> == 'success'`, the check #156 and #185 settled on) opens or updates one release PR holding the next version and the changelog, computed from the Conventional Commit subjects the `pr-title` check enforces. `plan-prod` posts the prod Terraform plan on that PR.
- **Merging the release PR** is a merge to `main`, so the same jobs run for that commit. `release-please` then creates the tag and the GitHub release, and `promote` calls `promote-release.yaml`, which applies prod Terraform and deploys backend, worker and frontend by that commit's SHA from prod's own stores. Nothing is rebuilt. A release is therefore only cut for a commit that is live and healthy on staging with its artifacts in both stores.
- **Rollback**, or finishing a promotion whose deploy failed, is `promote-release.yaml` run by hand (`workflow_dispatch`) with a release tag.
- **Tags** are `vMAJOR.MINOR.PATCH`. `feat:` bumps minor, `fix:` patch, a breaking change major; while the major version is 0 a breaking change bumps minor. `build:`, `chore:`, `ci:`, `docs:` and `refactor:` also bump patch (`changelog-sections` lists them as visible), so every change that can alter what runs, Renovate's `build(deps):` bumps included, gets a release PR and reaches production; release-please skips a commit type it hides. `style:` and `test:` still do not call for a release. The manifest starts at `0.31.4`, the last tag the old flow cut.
- **Token.** A PR opened with the default `GITHUB_TOKEN` does not trigger the PR checks, so release-please runs with the PAT in Secret Manager (`github_provider_token`), which therefore also needs Contents and Pull requests read/write, and is readable only by the deploy identity ([38](0038-separate-plan-and-apply-identities.md)).
- **Files.** `.github/release-please-config.json` and `.github/.release-please-manifest.json`; `CHANGELOG.md` at the root is maintained by each release PR, and release-please keeps its version in `.github/version.txt`. None is a language manifest, so [1](0001-manifests-live-with-their-code.md) is unaffected.
- **Staging** runs a commit, not a release, so its footer shows the short SHA, linked to the commit; production shows the release tag.
- **What a merge reuses** is diffed against the newest release tag, as before. Tags are now sparser than merges, so an unchanged service is re-tagged from the last release's artifacts rather than the previous merge's, and once a service has changed since the last release every merge rebuilds it until the next release.
- `apply-infrastructure-prod.yaml` stays as the manual backfill ([24](0024-manual-prod-infrastructure-apply.md)), and shares the promotion's concurrency group.

## Consequences

The releases page lists only versions deployed to production. Promotion is reviewable: the release PR carries the changelog and the plan. A failed deploy after the release is cut leaves a release that is not live, and is finished by running Promote Release with its tag.
