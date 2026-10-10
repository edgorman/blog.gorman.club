# 26. One GCP project per environment, plus a root

Status: Accepted

## Context

Staging must not be able to reach production resources by accident.

## Decision

- `blog-gorman-club-stag` and `blog-gorman-club-prod` are separate projects, and resources carry environment suffixes (`backend-stag`, `backend-prod`).
- `infrastructure/root` provisions `blog-gorman-club-root` and what both share: the two environment projects, every Terraform state bucket (root's included), the GitHub Actions WIF pool and provider, shared domain configuration, and the baseline APIs on all three projects.
- Root's first apply is the only manual one, against local state, since nothing has credentials or state before it. State is then migrated to the bucket it created (`terraform init -migrate-state`). After that root is planned on PR and applied on merge like the others.

## Consequences

Blast radius is a project boundary. Per-PR environments are a possible later addition, not needed yet (#273).
