# App-specific APIs; the baseline set shared by every project is enabled centrally by infrastructure/root.
resource "google_project_service" "run" {
  project = var.gcp_project_id
  service = "run.googleapis.com"
}

resource "google_project_service" "artifact_registry" {
  project = var.gcp_project_id
  service = "artifactregistry.googleapis.com"
}

# The Gemini Enterprise Agent Platform, which serves the models the writing assistant calls. Its
# API is still named aiplatform - the product was renamed, the service was not. The backend
# authenticates against it with its own runtime service account rather than an API key; the Gemini
# API (generativelanguage) cannot be reached that way at all, which is why this is the platform in
# use (see services/backend/internal/repository/gemini).
resource "google_project_service" "agent_platform" {
  project = var.gcp_project_id
  service = "aiplatform.googleapis.com"
}