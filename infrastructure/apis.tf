# App-specific APIs for this environment's project. The baseline set shared
# by every project (storage, secretmanager, iam, ...) is already enabled
# centrally by infrastructure/root (see gcp_project.tf there); Cloud Run and
# Artifact Registry are only needed where the backend actually runs, so they
# live here instead.
resource "google_project_service" "run" {
  project = var.gcp_project_id
  service = "run.googleapis.com"
}

resource "google_project_service" "artifact_registry" {
  project = var.gcp_project_id
  service = "artifactregistry.googleapis.com"
}
