# 42. Release commits have their own workflow

Status: Accepted (amends [39](0039-releases-from-release-please.md))

## Context

A push to `main` is one of two kinds: a release-please release commit (the merge of its release PR, which changes only `CHANGELOG.md`, `.github/.release-please-manifest.json` and `.github/version.txt`), or anything else. `push-commit.yaml` handled both. For a release commit it applied infrastructure, re-tagged three artifacts, redeployed the unchanged frontend to staging and ran `smoke` only to reach `release-please` and `promote`.

## Decision

GitHub path filters pick the workflow; moon is not involved.

- **`push-commit.yaml`** runs on every push except one that changes only those three files (`paths-ignore`). It applies, builds, deploys staging, smoke-tests, and runs `release-please` to open or update the release PR, then `plan-prod`. It never tags a release.
- **`push-release.yaml`** runs when a push changes any of those three files. It publishes the previous commit's (`github.event.before`) artifacts under the release commit, building them if they are missing, then runs `release-please` to tag and release, then `promote`. It does not touch staging, which already runs the same code. It has its own concurrency group so a later push can't supersede a waiting release.
- A push that changes both kinds of file runs both; `push-commit.yaml`'s `changed` job still diffs against the previous successful run or last release tag, so it is unaffected by a release commit having no run of its own.
- The deploy identity is bound to both filenames in `github_cicd.tf`, so root's apply must have run from `push-commit.yaml` before the first release commit. The merge that adds `push-release.yaml` does that.
- No required status check comes from either workflow (they run after merge), so a skipped one blocks nothing.
