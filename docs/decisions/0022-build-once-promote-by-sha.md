# 22. Build once, promote by commit SHA

Status: Accepted

## Context

Production should run exactly the bytes validated in staging, and a rollback should never depend on rebuilding an old commit.

## Decision

- Every merge to `main` builds the backend and worker images and pushes them, tagged with the commit SHA (never the version, which can still be renamed), to both the staging and prod Artifact Registries. This runs whether or not `services/backend/**` changed, so every release tag has an image in both. A merge whose run is cancelled while pending gets none; the next run covers its infrastructure.
- The frontend builds the same way, as a plain `dist/` with no backend URL baked in, uploaded under `<commit-sha>/` to both frontend buckets. Staging deploys from that same `dist/`.
- The backend URL is runtime configuration: `frontend-deploy` writes `config.json` next to the fetched folder (`services/frontend/src/lib/config.ts`), so the stored folder stays environment-neutral.
- Promoting a pre-release runs, in order: `terraform apply` on prod, `gcloud run deploy` of `backend:${{ github.sha }}` from prod's own registry, then the frontend folder from prod's own bucket (`frontend-fetch`) to Cloudflare Pages with `config.json`. No cross-project read, no copy, no rebuild.
- Promotion also tags the backend and worker images `release-<version>`, for retention only ([29](0029-artifact-retention.md)). Nothing deploys by it.
- Rollback is promoting an earlier tag: the same flow, with that release's infrastructure, image and folder, all already in place.

## Consequences

A store's size tracks merges, not deploys, which is why retention exists. Between the backend and frontend steps a new backend serves the old bundle ([10](0010-wire-changes-across-releases.md)).
