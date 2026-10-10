# 29. Artifact retention keeps recent merges and every release

Status: Accepted

## Context

Every merge writes to both projects' stores ([22](0022-build-once-promote-by-sha.md)), so they grow with merge volume, even across long gaps between promotions. Before this nothing deleted anything.

## Decision

Both rules apply identically in both projects (`infrastructure/env/artifact_registry.tf`, `storage.tf`):

- Registry: keep the `backend_registry_keep_count` (default 30) most recent versions of each image (`backend` and `worker`, one KEEP policy each by `package_name_prefixes`) and delete the rest, Artifact Registry's documented keep-N pattern. Thirty is counted in versions because rollback goes by release, not calendar. A third KEEP policy, `keep-releases`, pins every `release-` tagged version, since no policy can say "still referenced by a Cloud Run revision" (#279). Promotion adds that tag, so every image prod has run stays.
- Buckets: a `lifecycle_rule` deletes commit-SHA folders older than `frontend_retention_days` (default 90). GCS can only match by age, so it can't spare the live folder: promote, or raise the window, before a live folder ages out.
- Both start dry-run: `cleanup_policy_dry_run = true` on the registry, and a one-off listing for the bucket rule (`gcloud storage ls -L` filtered by creation time) reviewed before relying on it. Turning on real deletion is a separate, deliberate change.

## Consequences

The most-recent window still matters for staging and for commits never promoted.
