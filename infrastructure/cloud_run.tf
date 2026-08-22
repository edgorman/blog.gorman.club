resource "google_cloud_run_v2_service" "backend" {
  depends_on = [google_project_service.run]

  project  = var.gcp_project_id
  name     = "backend-${var.environment}"
  location = var.gcp_region
  ingress  = "INGRESS_TRAFFIC_ALL"

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
    # CI deploys new revisions directly (backend-deploy / backend-promote
    # composite actions) per the Staging Deployments / Production Releases
    # flow in CLAUDE.md - Terraform owns the service's existence and config,
    # never which image is currently live, so it must not fight those
    # deploys by planning to revert the image back to the placeholder above.
    ignore_changes = [template[0].containers[0].image]
  }
}

# The backend is a public API the frontend calls directly from the browser
# (see the Debug Endpoint Contract in CLAUDE.md) - it has no caller identity
# to authenticate, so it's deliberately open rather than restricted to a
# service-to-service invoker.
resource "google_cloud_run_v2_service_iam_member" "public" {
  project  = google_cloud_run_v2_service.backend.project
  location = google_cloud_run_v2_service.backend.location
  name     = google_cloud_run_v2_service.backend.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
