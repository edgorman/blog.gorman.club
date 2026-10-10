# 21. `.prototools` is the single source of tool versions

Status: Accepted

## Context

Versions used to live in several places: `bufbuild/buf-setup-action`'s `version:`, `setup-go`/`setup-node` steps, and local installs.

## Decision

The root `.prototools` pins `moon`, `go`, `node`, `npm`, `buf` and `terraform`, read by [proto](https://moonrepo.dev/proto) in CI (`moonrepo/setup-toolchain`) and locally (`proto install`). The `buf` and `terraform` plugins are pinned to a commit SHA; both verify checksums, so this pins where they're fetched from. `root:go-version-sync` fails CI if the `go` pin and `services/backend/go.mod` disagree; the Dockerfile's `golang` image moves with them (#123).

## Consequences

A version bump is one line, plus `go.mod` and the Dockerfile for Go.
