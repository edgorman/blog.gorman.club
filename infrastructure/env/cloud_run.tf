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

# CI needs actAs to attach this service account below; no role in github_cicd.tf covers it.
resource "google_service_account_iam_member" "backend_runtime_actas" {
  service_account_id = google_service_account.backend_runtime.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:github-actions@blog-gorman-club-root.iam.gserviceaccount.com"
}

resource "google_cloud_run_v2_service" "backend" {
  depends_on = [google_project_service.run]

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
