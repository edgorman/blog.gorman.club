# App-specific APIs; the baseline set shared by every project is enabled centrally by infrastructure/root.
resource "google_project_service" "run" {
  project = var.gcp_project_id
  service = "run.googleapis.com"
}

resource "google_project_service" "artifact_registry" {
  project = var.gcp_project_id
  service = "artifactregistry.googleapis.com"
}

# Gemini is called through Vertex AI rather than the Generative Language API, so the backend
# authenticates with its own runtime service account instead of an API key (see cloud_run.tf).
resource "google_project_service" "aiplatform" {
  project = var.gcp_project_id
  service = "aiplatform.googleapis.com"
}
