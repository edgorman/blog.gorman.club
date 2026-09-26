# The worker: a second Cloud Run service built from services/backend (cmd/worker), invoked only by
# Eventarc when a post is written or a comment is created. See the Worker section of
# services/backend/AGENTS.md.

resource "google_project_service" "eventarc" {
  project = var.gcp_project_id
  service = "eventarc.googleapis.com"
}

# Eventarc delivers Firestore events through a Pub/Sub topic and push subscription of its own.
resource "google_project_service" "pubsub" {
  project = var.gcp_project_id
  service = "pubsub.googleapis.com"
}

resource "google_service_account" "worker_runtime" {
  project      = var.gcp_project_id
  account_id   = "worker-runtime"
  display_name = "Worker Cloud Run runtime"
}

# The worker reads and writes the documents its events name, and later calls the Agent Platform
# (embeddings, moderation) the same way the backend's assistant does: as its runtime identity.
resource "google_project_iam_member" "worker_runtime_datastore_user" {
  project = var.gcp_project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.worker_runtime.email}"
}

resource "google_project_iam_member" "worker_runtime_agent_platform_user" {
  project = var.gcp_project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${google_service_account.worker_runtime.email}"
}

# The identity Eventarc invokes the worker as. It is the only principal with run.invoker on it.
resource "google_service_account" "worker_trigger" {
  project      = var.gcp_project_id
  account_id   = "worker-trigger"
  display_name = "Eventarc trigger invoking the worker"
}

resource "google_project_iam_member" "worker_trigger_event_receiver" {
  project = var.gcp_project_id
  role    = "roles/eventarc.eventReceiver"
  member  = "serviceAccount:${google_service_account.worker_trigger.email}"
}

# CI attaches worker_runtime to the service and worker_trigger to the triggers, and both need actAs.
resource "google_service_account_iam_member" "worker_actas" {
  for_each = {
    runtime = google_service_account.worker_runtime.name
    trigger = google_service_account.worker_trigger.name
  }

  service_account_id = each.value
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:github-actions@blog-gorman-club-root.iam.gserviceaccount.com"
}

resource "google_cloud_run_v2_service" "worker" {
  depends_on = [google_project_service.run]

  project  = var.gcp_project_id
  name     = "worker-${var.environment}"
  location = var.gcp_region
  # Eventarc's push subscription in this project counts as internal traffic; nothing from outside
  # the project reaches the service at all, authenticated or not.
  ingress = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  labels = {
    managed-by = "terraform"
  }

  template {
    service_account = google_service_account.worker_runtime.email

    scaling {
      min_instance_count = 0
      max_instance_count = var.worker_max_instances
    }

    containers {
      image = var.backend_initial_image

      ports {
        container_port = 8080
      }

      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }
    }
  }

  lifecycle {
    # CI deploys the real image, and gcloud stamps client/client_version - neither is real drift.
    ignore_changes = [
      template[0].containers[0].image,
      client,
      client_version,
    ]
  }
}

resource "google_cloud_run_v2_service_iam_member" "worker_trigger_invoker" {
  project  = google_cloud_run_v2_service.worker.project
  location = google_cloud_run_v2_service.worker.location
  name     = google_cloud_run_v2_service.worker.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.worker_trigger.email}"
}

locals {
  # One trigger per event the worker handles. Firestore events are regional, so each trigger lives
  # in the database's own location (var.gcp_region, see firestore.tf).
  worker_triggers = {
    blog-written = {
      type     = "google.cloud.firestore.document.v1.written"
      document = "blogs/{slug}"
      path     = "/events/blog"
    }
    comment-created = {
      type     = "google.cloud.firestore.document.v1.created"
      document = "blogs/{slug}/comments/{id}"
      path     = "/events/comment"
    }
  }
}

resource "google_eventarc_trigger" "worker" {
  for_each = local.worker_triggers

  depends_on = [
    google_project_service.eventarc,
    google_project_service.pubsub,
    google_project_iam_member.worker_trigger_event_receiver,
  ]

  project  = var.gcp_project_id
  name     = "worker-${each.key}"
  location = var.gcp_region

  matching_criteria {
    attribute = "type"
    value     = each.value.type
  }
  matching_criteria {
    attribute = "database"
    value     = google_firestore_database.database.name
  }
  matching_criteria {
    attribute = "document"
    value     = each.value.document
    operator  = "match-path-pattern"
  }

  # Firestore events are only offered as protobuf, and it must be said explicitly: the provider
  # sends an unset value as "", which the API rejects rather than defaulting. The worker reads the
  # event from its headers, not its body, so the encoding doesn't matter to it.
  event_data_content_type = "application/protobuf"
  service_account         = google_service_account.worker_trigger.email

  destination {
    cloud_run_service {
      service = google_cloud_run_v2_service.worker.name
      region  = var.gcp_region
      path    = each.value.path
    }
  }

  labels = {
    managed-by = "terraform"
  }
}

# The worker answers 5xx only for a failure it wants retried (internal/worker), so a run of them
# means events are piling up in Eventarc's retries rather than being handled. Measured on Cloud
# Run's own request count, like backend_error_rate in monitoring.tf.
resource "google_monitoring_alert_policy" "worker_error_rate" {
  depends_on = [google_project_service.monitoring]

  project      = var.gcp_project_id
  display_name = "worker-${var.environment} is failing events"
  combiner     = "OR"

  conditions {
    display_name = "5xx responses in a 5 minute window"

    condition_threshold {
      filter = join(" AND ", [
        "metric.type=\"run.googleapis.com/request_count\"",
        "resource.type=\"cloud_run_revision\"",
        "resource.label.service_name=\"${google_cloud_run_v2_service.worker.name}\"",
        "metric.label.response_code_class=\"5xx\"",
      ])

      comparison      = "COMPARISON_GT"
      threshold_value = var.alert_error_count_threshold
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
    subject   = "worker-${var.environment} failing events"
    mime_type = "text/markdown"
    content   = <<-EOT
      `worker-${var.environment}` failed more than ${var.alert_error_count_threshold} event
      deliveries in five minutes. Each failure is logged as "event failed, will be retried" with
      the event type and document path, and Eventarc keeps redelivering those events until they
      succeed or expire.
    EOT
  }

  alert_strategy {
    auto_close = "3600s"
  }
}
