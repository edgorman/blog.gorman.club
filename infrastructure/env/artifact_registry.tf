# One repository per environment project; project isolation already keeps staging/prod apart, so no environment suffix is needed here.
resource "google_artifact_registry_repository" "backend" {
  depends_on = [google_project_service.artifact_registry]

  project       = var.gcp_project_id
  location      = var.gcp_region
  repository_id = "backend"
  format        = "DOCKER"
}

# Stores the frontend image built alongside the Cloudflare Pages deploy; not served from here.
resource "google_artifact_registry_repository" "frontend" {
  depends_on = [google_project_service.artifact_registry]

  project       = var.gcp_project_id
  location      = var.gcp_region
  repository_id = "frontend"
  format        = "DOCKER"
}
