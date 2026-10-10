# 30. Monitoring watches Cloud Run's own metrics

Status: Accepted

## Context

`infrastructure/env/monitoring.tf` applies to both environments, so a noisy policy shows up in staging first.

## Decision

- The uptime check polls `/health` and matches the `"status":"ok"` body, since a 200 only proves something is listening. Its period is the longest offered (15 minutes): the service scales to zero, and frequent probes would keep an instance warm for no gain.
- Request alerts use Cloud Run metrics, not log-based ones, so nothing tracks the handlers: uptime failing from more than one region, more than N `5xx` in five minutes (counted, since rates are noise at this traffic), a `4xx` spike, any real number of `429`s, and p95 latency above a threshold well clear of a cold start.
- The assistant, the one route that spends money, writes exactly one `assistant turn` log line per turn (outcome, upstream status on failure, rounds, tokens; never content or a uid). Two log-based metrics count it, with alerts on turns per hour and on any failed turn. The filter matches the message exactly, so it changes with `assistantTurnMessage` in `internal/service/chat.go`.
- One `backend-<env>` dashboard per environment, every threshold a variable with a default and description, every policy with a `documentation` block.
- One email channel per address in `alert_notification_emails`, so a later policy can notify a subset. The list grants no access.

## Consequences

An alert always has a dashboard and a first place to look.
