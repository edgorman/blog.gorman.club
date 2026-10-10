# 27. CI authenticates with Workload Identity Federation

Status: Accepted

## Context

JSON service-account keys are long-lived secrets.

## Decision

- Workflows authenticate over short-lived OIDC tokens. One GitHub Actions service account lives in root and is granted roles on each project individually, never at org level. The provider's attribute condition admits only this repository's tokens.
- The root apply writes the provider path and service account email into GitHub Actions variables, so workflows never hardcode them.
- The one credential WIF can't replace, the GitHub PAT root's Terraform uses to write those variables, was supplied by hand once at bootstrap and is read back from the `github_provider_token` Secret Manager secret on each run, never stored as a GitHub secret.

## Consequences

No stored cloud credentials in GitHub. The assistant avoids API keys for the same reason ([11](0011-assistant-on-agent-platform.md)).
