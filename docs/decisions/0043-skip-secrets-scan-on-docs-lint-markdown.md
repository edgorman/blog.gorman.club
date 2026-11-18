# 43. Skip the secrets scan on docs, lint Markdown

Status: Accepted (amends [35](0035-advisory-security-scanning.md) and [41](0041-moon-gates-every-ci-job.md))

## Context

[41](0041-moon-gates-every-ci-job.md) let moon skip the scanners a change doesn't reach, but kept `gitleaks` on every change because any file can hold a secret. A docs-only pull request therefore still started a runner to scan prose. Markdown also had no check of its own beyond `root:decision-links`, so style and structure slips in docs went unnoticed.

## Decision

- `scan-secrets` no longer lists `docs/` or any `*.md` file as inputs, so a pull request that changes only those skips the `gitleaks` job. A change to code, config, a workflow, Terraform or any other file still scans the whole PR range, and the weekly and manual runs still scan all of `main`'s history.
- `root:markdown-lint` runs `markdownlint-cli2` over every Markdown file except the generated `CHANGELOG.md`. It is a `lint-check` target, so it runs when a Markdown file or `.markdownlint-cli2.jsonc` changes and is part of the existing required check. The rules are the defaults minus line length, inline HTML, first-line heading and table column style.

## Consequences

A secret pasted into a Markdown file in a PR is no longer caught on that PR. The weekly scan of `main` still finds it, late. Adding a rule to the lint means editing `.markdownlint-cli2.jsonc` and fixing what it reports in the same PR.
