# One repository per environment project; project isolation already keeps staging/prod apart, so no environment suffix is needed here.
#
# Every merge writes a commit-SHA image here in both projects alike (see Staging Deployments in
# CLAUDE.md), so this repository grows at the same rate whether or not the environment it lives in
# is deployed to often - prod's own registry fills up from merges even between rare promotions.
# Unbounded, that's one image per merge forever. The two cleanup_policies below are the standard
# Artifact Registry combination for "keep only the N most recent, delete everything else": the KEEP
# policy pins the most recent backend_registry_keep_count versions regardless of age, and the DELETE
# policy (unconditional - it matches any tag state) removes anything the KEEP policy didn't pin. See
# the Artifact Stores section of CLAUDE.md for how the count was chosen and what it does not
# guarantee (there is no policy condition for "a Cloud Run revision still references this").
#
# Ships with cleanup_policy_dry_run enabled: the policy evaluates and logs what it would delete
# without deleting anything, so the first real cleanup pass can be reviewed in Cloud Logging before
# a follow-up change flips this to false and lets it actually run.
resource "google_artifact_registry_repository" "backend" {
  depends_on = [google_project_service.artifact_registry]

  project       = var.gcp_project_id
  location      = var.gcp_region
  repository_id = "backend"
  format        = "DOCKER"

  cleanup_policy_dry_run = true

  cleanup_policies {
    id     = "keep-minimum-versions"
    action = "KEEP"

    most_recent_versions {
      keep_count = var.backend_registry_keep_count
    }
  }

  cleanup_policies {
    id     = "delete-the-rest"
    action = "DELETE"

    condition {
      tag_state = "ANY"
    }
  }
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
