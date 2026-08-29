# Dedicated runtime identity scoped to Firestore alone, not the default compute service account.
resource "google_service_account" "backend_runtime" {
  project      = var.gcp_project_id
  account_id   = "backend-runtime"
  display_name = "Backend Cloud Run runtime"
}

resource "google_project_iam_member" "backend_runtime_datastore_user" {
  project = var.gcp_project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.backend_runtime.email}"
}

# The writing assistant calls the Gemini API as this service account, which is the whole reason
# there is no API key anywhere in this deployment: the credential is the runtime identity itself,
# minted by the metadata server and short-lived, exactly as CI's is under WIF.
#
# A request authorized by a bearer token rather than an API key carries no project of its own, so
# the backend names the billing project in an x-goog-user-project header - and naming a project
# that way requires serviceusage.services.use on it, which is what this role grants and nothing
# broader does.
resource "google_project_iam_member" "backend_runtime_service_usage_consumer" {
  project = var.gcp_project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = "serviceAccount:${google_service_account.backend_runtime.email}"
}

# CI needs actAs to attach this service account below; no role in github_cicd.tf covers it.
resource "google_service_account_iam_member" "backend_runtime_actas" {
  service_account_id = google_service_account.backend_runtime.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:github-actions@blog-gorman-club-root.iam.gserviceaccount.com"
}

resource "google_cloud_run_v2_service" "backend" {
  depends_on = [google_project_service.run, google_project_service.generative_language]

  project  = var.gcp_project_id
  name     = "backend-${var.environment}"
  location = var.gcp_region
  ingress  = "INGRESS_TRAFFIC_ALL"

  labels = {
    managed-by = "terraform"
  }

  template {
    service_account = google_service_account.backend_runtime.email

    containers {
      image = var.backend_initial_image

      ports {
        container_port = 8080
      }

      env {
        name  = "ENVIRONMENT"
        value = var.environment
      }

      env {
        name  = "CORS_ALLOWED_ORIGIN"
        value = var.backend_cors_origin
      }

      env {
        name  = "GOOGLE_CLIENT_ID"
        value = var.google_client_id
      }

      # The project Gemini API requests are billed and rate-limited against. It is passed rather
      # than detected from the metadata server: Terraform already knows it, so a lookup would only
      # be a way of being told something this file could have said.
      env {
        name  = "GCP_PROJECT_ID"
        value = var.gcp_project_id
      }

      env {
        name  = "ASSISTANT_MODEL"
        value = var.assistant_model
      }

      # Who may use the assistant. It is plain configuration rather than a secret - it names
      # accounts, not credentials - so it lives in this environment's tfvars alongside everything
      # else that differs between staging and prod.
      env {
        name  = "ASSISTANT_ALLOWED_EMAILS"
        value = join(",", var.assistant_allowed_emails)
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

# Public API called directly from the browser, so it's deliberately open rather than invoker-restricted.
resource "google_cloud_run_v2_service_iam_member" "public" {
  project  = google_cloud_run_v2_service.backend.project
  location = google_cloud_run_v2_service.backend.location
  name     = google_cloud_run_v2_service.backend.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
