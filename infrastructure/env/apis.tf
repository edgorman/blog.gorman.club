# App-specific APIs; the baseline set shared by every project is enabled centrally by infrastructure/root.
resource "google_project_service" "run" {
  project = var.gcp_project_id
  service = "run.googleapis.com"
}

resource "google_project_service" "artifact_registry" {
  project = var.gcp_project_id
  service = "artifactregistry.googleapis.com"
}

# The Gemini API the writing assistant calls. The backend authenticates against it with its own
# runtime service account rather than an API key (see cloud_run.tf).
resource "google_project_service" "generative_language" {
  project = var.gcp_project_id
  service = "generativelanguage.googleapis.com"
}
