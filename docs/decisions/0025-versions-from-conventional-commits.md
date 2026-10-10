# 25. Versions come from Conventional Commit subjects

Status: Accepted

## Context

Every PR is squash-merged with its title as the commit subject (`.github/settings.yml`).

## Decision

Tags are plain `major.minor.patch`; pre-release status is GitHub's release flag, not a suffix. [`trunk-based-release-versioning`](https://github.com/Fresa/trunk-based-release-versioning) computes each bump from the subjects since the last tag: `feat:` is minor, `!` or `BREAKING CHANGE:` is major, anything else patch, the highest winning. A pre-release can still be renamed before promotion. The `pr-title` check enforces the format (#276).

## Consequences

The PR title decides the version, so it must be a Conventional Commit.
