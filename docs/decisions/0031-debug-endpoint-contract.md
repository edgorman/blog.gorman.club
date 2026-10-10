# 31. The debug endpoint contract

Status: Accepted, partly superseded

## Context

Before building features, setup was validated with a small debug contract: the backend's `/health` and `/debug` return status, timestamp, environment and commit SHA, and a frontend dashboard would fetch `/debug` so green indicators on `staging.blog.gorman.club` and `blog.gorman.club` proved DNS, CORS, environment variables and IAM worked across both clouds.

## Decision

The backend half shipped (`internal/service/debug.go`). The dashboard half was never built: the GCP uptime check ([30](0030-monitoring-on-cloud-run-metrics.md)) took its place and matches the literal `"status":"ok"` substring.

## Consequences

`debugResponse` is the one allowlisted hand-written wire type ([9](0009-no-hand-written-wire-types.md)): nothing else declares the shape to drift against.
