# infrastructure

Terraform roots: `env` (applied per environment from `config/staging` and `config/prod`) and `root` (shared, bootstrapped once). How they are planned and applied is in `.github/AGENTS.md`. Reasoning for each rule is in the linked record under `docs/decisions/`.

## Cloud Infrastructure & Security Isolation

- Staging and prod are separate GCP projects (`blog-gorman-club-stag`, `blog-gorman-club-prod`), and every resource carries its environment suffix (`backend-stag`) ([26](../docs/decisions/0026-one-gcp-project-per-environment.md)).

### Root Environment

- `infrastructure/root` owns the root project, both environment projects, every state bucket, the WIF pool and provider, shared DNS and the baseline APIs. Its first apply was the only manual one; everything since goes through CI ([26](../docs/decisions/0026-one-gcp-project-per-environment.md)).

### IAM & Authentication

- CI authenticates with Workload Identity Federation. No JSON keys, and roles are granted per project, never at org level ([27](../docs/decisions/0027-workload-identity-federation.md)).
- Workflows read the WIF provider and service account from repository variables written by root; never hardcode them ([27](../docs/decisions/0027-workload-identity-federation.md)).
- The GitHub PAT lives in Secret Manager (`github_provider_token`), never in a GitHub secret ([27](../docs/decisions/0027-workload-identity-federation.md)).

### Artifact Stores

- Each environment has its own registry (`backend`) and bucket (`<gcp-project-id>-frontend`), written once by the job that built the artifact and read only within that project ([28](../docs/decisions/0028-per-environment-artifact-stores.md)).
- Retention: keep the newest `backend_registry_keep_count` versions per image plus every `release-` tag; delete frontend folders after `frontend_retention_days`. Both stay dry-run until the output is reviewed ([29](../docs/decisions/0029-artifact-retention.md)).

## Monitoring & Alerting

- Watch both environments with the same `env/monitoring.tf`; alert on Cloud Run metrics, not log-based ones, except for the assistant ([30](../docs/decisions/0030-monitoring-on-cloud-run-metrics.md)).
- The uptime check matches `"status":"ok"` on `/health` every 15 minutes ([30](../docs/decisions/0030-monitoring-on-cloud-run-metrics.md)).
- The `assistant turn` log metric filter and `assistantTurnMessage` in `internal/service/chat.go` change together ([30](../docs/decisions/0030-monitoring-on-cloud-run-metrics.md)).
- Every threshold is a variable in `env/variables.tf` with a default and description, and every policy has a `documentation` block ([30](../docs/decisions/0030-monitoring-on-cloud-run-metrics.md)).
