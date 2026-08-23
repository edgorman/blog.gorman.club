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
    }
  }

  lifecycle {
    # CI deploys the real image directly; Terraform must not revert it to the placeholder above.
    # client/client_version are stamped by `gcloud run deploy` itself and aren't meaningful drift.
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
