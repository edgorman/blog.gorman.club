# Monitoring for the backend: an uptime check against the /health endpoint of the Debug Endpoint
# Contract, alert policies on that check and on Cloud Run's own request metrics, and an optional
# budget alert on the project's spend. Shared by both environments, so staging and prod are
# watched the same way and a policy that turns out to be noisy is discovered in staging first.

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

# The budget's project filter takes a project number, which is not something this configuration is
# told - only the id is.
data "google_project" "environment" {
  project_id = var.gcp_project_id
}

# Cost, unlike everything above, is not a project-level resource: a budget belongs to the billing
# account, so creating one needs billing.budgets.create on that account and no project role grants
# it. That is why this is switched off by default - see the note on budget_amount in variables.tf
# for the one-time grant that turns it on.
resource "google_billing_budget" "environment" {
  # budget_amount is the only switch; that it cannot be set without a billing account to
  # charge against is enforced where it is declared, so a missing one fails loudly rather than
  # quietly leaving the budget uncreated.
  count = var.budget_amount > 0 ? 1 : 0

  billing_account = var.billing_account
  display_name    = "${var.gcp_project_id} monthly budget"

  budget_filter {
    projects = ["projects/${data.google_project.environment.number}"]
  }

  amount {
    specified_amount {
      # currency_code is deliberately unset: a budget must be denominated in the billing account's
      # own currency, so naming one here could only ever disagree with it.
      units = tostring(var.budget_amount)
    }
  }

  # Spend so far, at the points where the news is still useful rather than historical.
  threshold_rules {
    threshold_percent = 0.5
  }

  threshold_rules {
    threshold_percent = 0.9
  }

  threshold_rules {
    threshold_percent = 1.0
  }

  # And the one rule that can arrive before the money is gone: a forecast of overrunning the month.
  threshold_rules {
    threshold_percent = 1.0
    spend_basis       = "FORECASTED_SPEND"
  }

  # Sent to the same channels as the alerts above, so cost arrives where availability does. The
  # billing account's own admins keep receiving it too, which is the default and left alone: they
  # are the people who can act on a bill, whoever happens to be listed here.
  dynamic "all_updates_rule" {
    for_each = length(local.notification_channels) > 0 ? [1] : []

    content {
      monitoring_notification_channels = local.notification_channels
    }
  }
}
