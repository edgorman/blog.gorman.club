# Monitoring for the backend: an uptime check against the /health endpoint of the Debug Endpoint
# Contract, alert policies on that check, on Cloud Run's own request metrics and on the writing
# assistant's per-turn log line, and a dashboard showing all of it. Shared by
# both environments, so staging and prod are watched the same way and a policy that turns out to
# be noisy is discovered in staging first.

resource "google_project_service" "monitoring" {
  project = var.gcp_project_id
  service = "monitoring.googleapis.com"
}

# Where every alert in this file is sent. One channel per address rather than one channel holding
# several: a channel is what a policy references, so keeping them separate is what lets a future
# policy notify a subset without a second channel being created for the same address.
resource "google_monitoring_notification_channel" "email" {
  depends_on = [google_project_service.monitoring]

  for_each = toset(var.alert_notification_emails)

  project      = var.gcp_project_id
  display_name = "Email ${each.value} (${var.environment})"
  type         = "email"

  labels = {
    email_address = each.value
  }
}

locals {
  notification_channels = [for c in google_monitoring_notification_channel.email : c.id]

  # The uptime check addresses the Cloud Run service by hostname, so it follows the service rather
  # than a URL written down here; monitored_resource wants the bare host, without the scheme.
  backend_host = trimprefix(google_cloud_run_v2_service.backend.uri, "https://")
}

# Polls /health from Google's checker regions. This is the first thing in the deployment that
# actually exercises the Debug Endpoint Contract - until now it was only ever read by hand from
# the frontend dashboard.
resource "google_monitoring_uptime_check_config" "backend_health" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} health"
  timeout      = "10s"
  # The longest period the check offers, deliberately: the service scales to zero, so every probe
  # is a request that may cold-start an instance, and a minute-by-minute check would keep one warm
  # around the clock for no diagnostic gain on a blog.
  period = "900s"

  monitored_resource {
    type = "uptime_url"

    labels = {
      project_id = var.gcp_project_id
      host       = local.backend_host
    }
  }

  http_check {
    path           = "/health"
    port           = 443
    use_ssl        = true
    validate_ssl   = true
    request_method = "GET"

    accepted_response_status_codes {
      status_class = "STATUS_CLASS_2XX"
    }
  }

  # A 200 alone only proves something is listening on the hostname; the body is what proves the
  # backend itself answered. Matched on the status field rather than the whole payload because the
  # timestamp and commit fields change on every request by design.
  content_matchers {
    content = "\"status\":\"ok\""
    matcher = "CONTAINS_STRING"
  }
}

# Fires when the check fails from more than one region at once. More than one is the point: a
# single failing checker is far more likely to be that checker than the service, and alerting on
# it would teach the recipient to ignore this alert.
resource "google_monitoring_alert_policy" "backend_uptime" {
  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is failing its health check"
  combiner     = "OR"

  conditions {
    display_name = "Uptime check failing"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\"",
        "resource.type=\"uptime_url\"",
        "metric.label.check_id=\"${google_monitoring_uptime_check_config.backend_health.uptime_check_id}\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = 1
      duration        = "60s"

      aggregations {
        # ALIGN_NEXT_OLDER carries the last result forward across the gaps between probes, and
        # REDUCE_COUNT_FALSE then counts how many regions that result was a failure in.
        alignment_period     = "1200s"
        per_series_aligner   = "ALIGN_NEXT_OLDER"
        cross_series_reducer = "REDUCE_COUNT_FALSE"
        group_by_fields      = ["resource.label.host"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} health check failing"
    mime_type = "text/markdown"
    content   = <<-EOT
      The uptime check against `https://${local.backend_host}/health` is failing from more than one
      region. Either the Cloud Run service is down or it is answering with something other than the
      `{"status":"ok",...}` body of the Debug Endpoint Contract.

      Start with the Cloud Run revision logs for `backend-${var.environment}`: a failing deploy
      leaves the previous revision serving traffic, so a red check here usually means the running
      revision itself broke rather than that a deploy was rejected.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# Cloud Run counts every response by class, so 5xx is measured here rather than derived from logs -
# there is no log-based metric to define and keep in step with the handlers.
resource "google_monitoring_alert_policy" "backend_error_rate" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is returning 5xx responses"
  combiner     = "OR"

  conditions {
    display_name = "5xx responses in a 5 minute window"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
        "metric.label.response_code_class=\"5xx\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_error_count_threshold
      # The window itself is the duration; requiring the count to stay high for a further period
      # would only delay an alert that a single window has already established.
      duration = "0s"

      aggregations {
        # A count over the window rather than a rate: on a site this quiet a rate reads as a
        # fraction too small to hold in your head, where "more than N errors in five minutes" is
        # a threshold that can be argued about.
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} serving 5xx responses"
    mime_type = "text/markdown"
    content   = <<-EOT
      `backend-${var.environment}` returned more than ${var.alert_error_count_threshold} server
      errors in five minutes. The handlers only answer 5xx when a repository call or the assistant
      fails, so the Cloud Run logs for the current revision name the cause directly.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# Latency is measured at the 95th percentile so a single slow request cannot raise it, and the
# threshold is set well above a cold start: the service scales to zero, so the first request after
# an idle spell legitimately takes seconds and alerting under that would page for normal behaviour.
resource "google_monitoring_alert_policy" "backend_latency" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is responding slowly"
  combiner     = "OR"

  conditions {
    display_name = "95th percentile request latency"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_latencies\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_latency_threshold_ms
      # Held for two windows, so a single burst of cold starts clears itself rather than alerting.
      duration = "600s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_PERCENTILE_95"
        cross_series_reducer = "REDUCE_MAX"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} responding slowly"
    mime_type = "text/markdown"
    content   = <<-EOT
      The 95th percentile latency of `backend-${var.environment}` has been above
      ${var.alert_latency_threshold_ms}ms for ten minutes, which is longer than a cold start
      explains. Firestore queries and the assistant's model call are the two things here slow
      enough to account for it.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# Cloud Run's own 4xx count, for the one class of client error that is not browsing: a flood of
# them is someone probing with bad tokens or hammering the API into the rate limiter. The threshold
# is high on purpose - a 404 from a mistyped or stale link is ordinary traffic and alerting on a
# handful of them would teach the recipient to ignore this alert.
resource "google_monitoring_alert_policy" "backend_client_errors" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is returning a spike of 4xx responses"
  combiner     = "OR"

  conditions {
    display_name = "4xx responses in a 5 minute window"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
        "metric.label.response_code_class=\"4xx\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_client_error_count_threshold
      duration        = "0s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} serving a spike of 4xx responses"
    mime_type = "text/markdown"
    content   = <<-EOT
      `backend-${var.environment}` returned more than ${var.alert_client_error_count_threshold}
      client errors in five minutes. Break the request count down by `response_code` in Metrics
      Explorer first: a wall of `401`s is someone trying tokens, `429`s are the rate limiter turning
      a caller away (see the separate rate limiting alert), and `404`s on paths the API does not
      serve are a scanner.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# The narrower half of the one above: a 429 is only ever the backend's own rate limiter refusing a
# caller (internal/service/ratelimit.go), so any real number of them means someone is sending far
# more than a person at an editor or a reader on a page does.
resource "google_monitoring_alert_policy" "backend_rate_limited" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} is rate limiting callers"
  combiner     = "OR"

  conditions {
    display_name = "429 responses in a 5 minute window"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
        # response_code is an INT64 label, so it is compared as a number rather than a string.
        "metric.label.response_code=429",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_rate_limited_count_threshold
      duration        = "0s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} rate limiting callers"
    mime_type = "text/markdown"
    content   = <<-EOT
      `backend-${var.environment}` refused more than ${var.alert_rate_limited_count_threshold}
      requests with `429` in five minutes, which is its rate limiter at work: someone is calling
      the API far faster than using the site would. The limiter already bounds each caller on each
      instance, so this is a notice rather than an outage. The request logs for the window show
      which paths and which client addresses the refused requests came from.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# The assistant is the one route that spends money, and nothing Cloud Run measures can tell one of
# its turns from any other request. So these two metrics count the single structured line the
# backend writes for every turn (logAssistantTurn in internal/service/chat.go) - the exception to
# preferring a platform metric above, because the platform's per-model metrics are not something
# this file could confirm the names and labels of before relying on them, and the log line carries
# exactly the outcome and upstream status the alerts below need. The filter matches that line's
# message exactly, so it and assistantTurnMessage in chat.go have to change together.
locals {
  assistant_turn_filter = join(" AND ", [
    "resource.type=\"cloud_run_revision\"",
    "resource.labels.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
    "jsonPayload.message=\"assistant turn\"",
  ])
}

resource "google_logging_metric" "assistant_turns" {
  project     = var.gcp_project_id
  name        = "backend/assistant_turns"
  description = "Writing assistant turns, by outcome and, for failed ones, the model API's HTTP status (0 when it never answered with one)."
  filter      = local.assistant_turn_filter

  metric_descriptor {
    metric_kind  = "DELTA"
    value_type   = "INT64"
    unit         = "1"
    display_name = "Assistant turns"

    labels {
      key         = "outcome"
      value_type  = "STRING"
      description = "ok, error, or canceled (the caller went away mid-turn)"
    }

    labels {
      key         = "upstream_status"
      value_type  = "STRING"
      description = "The model API's HTTP status on a failed turn, 0 when it answered with none; empty on a successful turn"
    }
  }

  label_extractors = {
    outcome         = "EXTRACT(jsonPayload.outcome)"
    upstream_status = "EXTRACT(jsonPayload.upstream_status)"
  }
}

# Tokens per turn as a distribution, for the dashboard: how large turns are, and whether that is
# drifting, is what explains a bill that grew faster than the turn count did.
resource "google_logging_metric" "assistant_tokens" {
  project     = var.gcp_project_id
  name        = "backend/assistant_tokens"
  description = "Total tokens the model reported for each writing assistant turn, summed over its tool-calling rounds."
  filter      = local.assistant_turn_filter

  metric_descriptor {
    metric_kind  = "DELTA"
    value_type   = "DISTRIBUTION"
    unit         = "1"
    display_name = "Assistant tokens per turn"
  }

  value_extractor = "EXTRACT(jsonPayload.total_tokens)"

  # 100 tokens doubling up to about 1.6M, which covers a one-line answer through a six-round turn
  # carrying a long post in every round.
  bucket_options {
    exponential_buckets {
      num_finite_buckets = 14
      growth_factor      = 2
      scale              = 100
    }
  }
}

locals {
  assistant_turns_metric  = "logging.googleapis.com/user/${google_logging_metric.assistant_turns.name}"
  assistant_tokens_metric = "logging.googleapis.com/user/${google_logging_metric.assistant_tokens.name}"
}

# Usage over an hour rather than five minutes: a runaway client or a tool-calling loop shows as a
# sustained rate, and an hour is long enough that one author's burst of edits does not look like
# one. The per-account rate limiter bounds each caller on each instance; this is the total.
resource "google_monitoring_alert_policy" "assistant_usage" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} writing assistant usage is unusually high"
  combiner     = "OR"

  conditions {
    display_name = "Assistant turns in an hour"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"${local.assistant_turns_metric}\"",
        "resource.type=\"cloud_run_revision\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_assistant_turn_threshold
      duration        = "0s"

      aggregations {
        alignment_period     = "3600s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["resource.label.service_name"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    subject   = "backend-${var.environment} writing assistant usage high"
    mime_type = "text/markdown"
    content   = <<-EOT
      The writing assistant on `backend-${var.environment}` ran more than
      ${var.alert_assistant_turn_threshold} turns in an hour, more than one author at work
      accounts for. Every turn is a billed model call. Start with the `assistant turn` log lines
      for the window: their `rounds` field shows a tool-calling loop (turns stuck near the limit
      of six), and the request logs of `POST /blogs/{slug}/chat` show whether one caller is
      responsible.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# Any failed model call alerts, separately from the general 5xx count: a turn that fails answers
# 502, which the 5xx policy only notices after several, and it cannot say why. Grouping by the
# upstream status keeps it on the incident, so the notification names whether it was a quota
# (429), a permission (403) or a request the API rejected (400).
resource "google_monitoring_alert_policy" "assistant_errors" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "backend-${var.environment} writing assistant calls are failing"
  combiner     = "OR"

  conditions {
    display_name = "Failed assistant turns in a 5 minute window"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"${local.assistant_turns_metric}\"",
        "resource.type=\"cloud_run_revision\"",
        "metric.label.outcome=\"error\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_assistant_error_count_threshold
      duration        = "0s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_DELTA"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields      = ["metric.label.upstream_status"]
      }

      trigger {
        count = 1
      }
    }
  }

  notification_channels = local.notification_channels

  documentation {
    # $${...} is Terraform's escape for a literal ${...}: these are Cloud Monitoring's variables,
    # filled in from the failing series when the notification is sent.
    subject   = "backend-${var.environment} writing assistant failing (upstream status $${metric.label.upstream_status})"
    mime_type = "text/markdown"
    content   = <<-EOT
      Writing assistant turns on `backend-${var.environment}` are failing, and the model API
      answered with status **$${metric.label.upstream_status}**. `429` is quota, `403` is a missing
      role or a disabled API, `400` is a request the API rejected, `404` is a model id
      (`assistant_model`) this location does not serve, and `0` means no HTTP answer at all: a
      timeout, a network failure, missing credentials, or a response with no candidate. The
      `assistant turn` lines at `ERROR` in the Cloud Run logs carry the reason enum in their
      `error` field.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}

# One place each alert above can be clicked through to, showing the same signals side by side so
# an assistant failure can be read against the request volume and instance count around it.
resource "google_monitoring_dashboard" "backend" {
  depends_on = [google_project_service.monitoring]

  project = var.gcp_project_id
  dashboard_json = jsonencode({
    displayName = "backend-${var.environment}"
    mosaicLayout = {
      columns = 12
      tiles = [for i, widget in [
        {
          title = "Requests by response class"
          xyChart = {
            dataSets = [{
              plotType = "STACKED_BAR"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = join(" AND ", [
                    "metric.type=\"run.googleapis.com/request_count\"",
                    "resource.type=\"cloud_run_revision\"",
                    "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
                  ])
                  aggregation = {
                    alignmentPeriod    = "300s"
                    perSeriesAligner   = "ALIGN_DELTA"
                    crossSeriesReducer = "REDUCE_SUM"
                    groupByFields      = ["metric.label.response_code_class"]
                  }
                }
              }
            }]
          }
        },
        {
          title = "95th percentile latency (ms)"
          xyChart = {
            dataSets = [{
              plotType = "LINE"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = join(" AND ", [
                    "metric.type=\"run.googleapis.com/request_latencies\"",
                    "resource.type=\"cloud_run_revision\"",
                    "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
                  ])
                  aggregation = {
                    alignmentPeriod    = "300s"
                    perSeriesAligner   = "ALIGN_PERCENTILE_95"
                    crossSeriesReducer = "REDUCE_MAX"
                    groupByFields      = ["resource.label.service_name"]
                  }
                }
              }
            }]
            thresholds = [{ value = var.alert_latency_threshold_ms }]
          }
        },
        {
          title = "Assistant turns by outcome"
          xyChart = {
            dataSets = [{
              plotType = "STACKED_BAR"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = "metric.type=\"${local.assistant_turns_metric}\" AND resource.type=\"cloud_run_revision\""
                  aggregation = {
                    alignmentPeriod    = "300s"
                    perSeriesAligner   = "ALIGN_DELTA"
                    crossSeriesReducer = "REDUCE_SUM"
                    groupByFields      = ["metric.label.outcome"]
                  }
                }
              }
            }]
          }
        },
        {
          title = "Assistant failures by upstream status"
          xyChart = {
            dataSets = [{
              plotType = "STACKED_BAR"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = "metric.type=\"${local.assistant_turns_metric}\" AND resource.type=\"cloud_run_revision\" AND metric.label.outcome=\"error\""
                  aggregation = {
                    alignmentPeriod    = "300s"
                    perSeriesAligner   = "ALIGN_DELTA"
                    crossSeriesReducer = "REDUCE_SUM"
                    groupByFields      = ["metric.label.upstream_status"]
                  }
                }
              }
            }]
          }
        },
        {
          title = "Assistant tokens per turn (median and 95th percentile)"
          xyChart = {
            dataSets = [for percentile in ["50", "95"] : {
              plotType       = "LINE"
              legendTemplate = "p${percentile}"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = "metric.type=\"${local.assistant_tokens_metric}\" AND resource.type=\"cloud_run_revision\""
                  aggregation = {
                    alignmentPeriod    = "3600s"
                    perSeriesAligner   = "ALIGN_DELTA"
                    crossSeriesReducer = "REDUCE_PERCENTILE_${percentile}"
                  }
                }
              }
            }]
          }
        },
        {
          title = "Instances"
          xyChart = {
            dataSets = [{
              plotType = "STACKED_AREA"
              timeSeriesQuery = {
                timeSeriesFilter = {
                  filter = join(" AND ", [
                    "metric.type=\"run.googleapis.com/container/instance_count\"",
                    "resource.type=\"cloud_run_revision\"",
                    "resource.label.service_name=\"${google_cloud_run_v2_service.backend.name}\"",
                  ])
                  aggregation = {
                    alignmentPeriod    = "300s"
                    perSeriesAligner   = "ALIGN_MAX"
                    crossSeriesReducer = "REDUCE_SUM"
                    groupByFields      = ["metric.label.state"]
                  }
                }
              }
            }]
          }
        },
        ] : {
        # Two tiles to a row, each half the width.
        xPos   = (i % 2) * 6
        yPos   = floor(i / 2) * 4
        width  = 6
        height = 4
        widget = widget
      }]
    }
  })
}
