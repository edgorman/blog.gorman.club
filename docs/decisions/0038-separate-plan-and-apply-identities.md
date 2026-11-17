# 38. Plan and apply authenticate as separate identities

Status: Accepted (supersedes [27](0027-workload-identity-federation.md))

## Context

One GitHub Actions service account held `roles/editor`, IAM admin and more on root, staging and prod, and the Workload Identity provider admitted any token from this repository. So any run, on any branch and event, could act as an IAM admin on prod, including a `pull_request` plan job running whatever workflow the PR branch carried. Review protects `main`; it did not protect the credentials. GitHub Environments would gate this, but their rules are unavailable on a private repository on GitHub Free, so the split is enforced on the Google side from claims in the OIDC token.

## Decision

- **Four identities** live in root, with roles granted per project, never at org level, and no JSON keys:
  - `github-plan`: `roles/viewer`, `roles/iam.securityReviewer` and `serviceUsageConsumer` on every project, and nothing writable. Any run in this repository can assume it, which is what the PR checks use.
  - `github-deploy`: the apply roles on every project, for `push-commit.yaml` as it exists on `main`: root and staging applies, builds and publishing to both stores, staging deploys, and reading the tokens release-please and Cloudflare need.
  - `github-promote`: the apply roles on prod only, plus prod's state bucket and the Cloudflare token, for `promote-release.yaml` and `apply-infrastructure-prod.yaml` as they exist on `main`.
  - `github-renovate`: the GitHub PAT secret and nothing else, for `renovate.yaml` as it exists on `main`. It runs a third-party action, so it is not given an apply identity ([40](0040-self-hosted-renovate.md)).
- **Binding.** The provider maps `attribute.job_workflow_ref` and the apply identities are bound to `<repo>/.github/workflows/<file>@refs/heads/main`. A pull request's runs carry `refs/pull/<n>/merge`, so a workflow edited on a PR branch cannot assume them. The provider's attribute condition still admits only this repository.
- **No secrets for a PR.** A pull request's workflow runs with the plan identity, so nothing it can read may be a credential. PR plans are `-refresh=false` against placeholder tokens (a 40-character one for Cloudflare, which validates its shape, and an empty one for GitHub, which looks its owner up on configure and rejects a bad token but not an anonymous call), so no provider makes an authenticated call, and the plan identity has no Secret Manager access. `secret_data` of the stored tokens is ignored by Terraform, so the placeholders never show as a change; rotating a token means adding a Secret Manager version, which workflows read as `latest`. The Cloudflare account and zone ids are not secrets and are set in `terraform.tfvars`; the zone is a variable, not a data source, so a plan never calls Cloudflare.
- The root apply writes the provider path, the four service account emails and the ids workflows need into GitHub Actions variables (`PLAN_`, `DEPLOY_`, `PROMOTE_` and `RENOVATE_SERVICE_ACCOUNT`, `CLOUDFLARE_ACCOUNT_ID`), so workflows never hardcode them. Until that first apply the infrastructure and deploy jobs skip.
- The environment stack receives the identity that deploys to it as `ci_service_account` (deploy for staging, promote for prod), for the `actAs` and store-writer bindings beside the resources they apply to.
- The original `github-actions` account stays for one release (`github_actions_legacy.tf`, the `SERVICE_ACCOUNT` variable) so the old workflows still authenticate between root's apply and the merge. A follow-up deletes it once `push-commit.yaml` has run on `main` as the deploy identity.
- The GitHub PAT, which WIF can't replace, is supplied by hand once at bootstrap and read back from the `github_provider_token` secret by deploy and renovate only.

## Consequences

- Prod's deploy credentials are only obtainable by the promotion workflows as they exist on `main`.
- Root owns IAM for every project, so the deploy identity can change IAM on prod too: the split keeps prod's credentials from everything but `main`'s workflows, not from a change merged to `main`. On a repository without branch protection, anything that can push to `main` can change those workflows.
- A workflow that needs an identity other than its own must call a reusable workflow, whose `job_workflow_ref` is its own file (`push-commit.yaml` calls `promote-release.yaml`).
- Bootstrap order matters: the identities and variables must exist (a local root apply) before the workflows that use them are merged.
