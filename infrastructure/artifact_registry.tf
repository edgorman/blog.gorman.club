# One "backend" repository per environment project - project isolation
# already keeps staging and prod images apart, per CLAUDE.md's Resource
# Naming section, so the repository itself doesn't need an environment
# suffix the way the Cloud Run service below does.
resource "google_artifact_registry_repository" "backend" {
  depends_on = [google_project_service.artifact_registry]

  project       = var.gcp_project_id
  location      = var.gcp_region
  repository_id = "backend"
  format        = "DOCKER"
}
