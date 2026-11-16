# 35. Security scanning is advisory

Status: Accepted

## Context

CI checked formatting, lint, types, tests and API drift, but nothing scanned for vulnerabilities or misconfiguration beyond Dependabot alerts: no static security analysis, no workflow or Terraform linting, no secret scan and no image scan. GitHub's own offerings for this (code scanning with CodeQL, secret scanning and push protection) depend on the plan, and open-source CLIs run as ordinary jobs on any of them.

## Decision

- `security.yaml` runs open-source CLIs as ordinary jobs, on every PR, weekly on `main` (new advisories land without a commit) and on demand: `govulncheck` and `gosec` for Go, `npm audit` for the frontend, `actionlint` and `zizmor` for workflows, `tflint` and `trivy config` for Terraform, `trivy image` on the built backend image, and `gitleaks` over a PR's commits or `main`'s history.
- Scanning is advisory. Each tool's findings go to the job summary and a warning annotation, and never fail the run; none is a required check. `.github/scripts/advisory.sh` wraps each one: the tool's own "found something" exit code is a warning, while any other non-zero exit is a failure, so a scanner that cannot run is noticed rather than read as a clean result.
- Go tools are installed with `go install <module>@<version>` and the rest are pinned in the workflow, so a new rule cannot turn the workflow red unannounced.

## Consequences

Findings are read in the job summary, not the Security tab, and nothing forces them to be fixed. Tightening a scanner into a gate is a deliberate later change: add it to `settings.yml`'s required checks and drop its advisory wrapper.
