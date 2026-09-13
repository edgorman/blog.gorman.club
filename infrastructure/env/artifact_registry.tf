# One repository per environment project; project isolation already keeps staging/prod apart, so no environment suffix is needed here.
resource "google_artifact_registry_repository" "backend" {
  depends_on = [google_project_service.artifact_registry]

  project       = var.gcp_project_id
  location      = var.gcp_region
  repository_id = "backend"
  format        = "DOCKER"
}

# Granted on the repository itself rather than relying on the project-wide editor role from
# infrastructure/root, per the IAM section of CLAUDE.md: a grant lives beside the resource it
# applies to. Applied in both environments - prod is the new one, since the merge-to-main job is
# moving from staging-only plus a release-time gcrane copy to publishing both on merge.
resource "google_artifact_registry_repository_iam_member" "backend_github_actions_writer" {
  project    = google_artifact_registry_repository.backend.project
  location   = google_artifact_registry_repository.backend.location
  repository = google_artifact_registry_repository.backend.name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:github-actions@blog-gorman-club-root.iam.gserviceaccount.com"
}

# Stores the frontend image built alongside the Cloudflare Pages deploy; not served from here.
resource "google_artifact_registry_repository" "frontend" {
  depends_on = [google_project_service.artifact_registry]

  project       = var.gcp_project_id
  location      = var.gcp_region
  repository_id = "frontend"
  format        = "DOCKER"
}
