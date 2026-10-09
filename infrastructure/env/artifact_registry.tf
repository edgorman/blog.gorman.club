# One repository per environment project; project isolation already keeps staging/prod apart, so no environment suffix is needed here.
#
# Every merge writes a commit-SHA image here in both projects alike (see Staging Deployments in
# `.github/AGENTS.md`), so this repository grows at the same rate whether or not the environment it lives in
# is deployed to often - prod's own registry fills up from merges even between rare promotions.
# Unbounded, that's one image per merge forever. The cleanup_policies below are the standard
# Artifact Registry combination for "keep only the N most recent, delete everything else": the KEEP
# policies pin the most recent backend_registry_keep_count versions regardless of age (plus every
# release-tagged image), and the DELETE policy (unconditional - it matches any tag state) removes
# anything a KEEP policy didn't pin. See
# the Artifact Stores section of `infrastructure/AGENTS.md` for how the count was chosen and what it does not
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

  # One KEEP policy per image, so each of backend and worker keeps its own
  # backend_registry_keep_count versions however the count is applied across packages (#264).
  cleanup_policies {
    id     = "keep-minimum-versions"
    action = "KEEP"

    most_recent_versions {
      package_name_prefixes = ["backend"]
      keep_count            = var.backend_registry_keep_count
    }
  }

  cleanup_policies {
    id     = "keep-minimum-worker-versions"
    action = "KEEP"

    most_recent_versions {
      package_name_prefixes = ["worker"]
      keep_count            = var.backend_registry_keep_count
    }
  }

  # Every image promoted to prod, whatever its age: promote-release.yaml tags it release-<version>
  # (e.g. release-v1.4.0) on top of its commit-SHA tag, so a rollback target never falls out of the
  # most-recent window above however many merges land between promotions (#279).
  cleanup_policies {
    id     = "keep-releases"
    action = "KEEP"

    condition {
      tag_state    = "TAGGED"
      tag_prefixes = ["release-"]
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
# infrastructure/root, per the IAM section of `infrastructure/AGENTS.md`: a grant lives beside the resource it
# applies to. Applied in both environments - prod is the new one, since the merge-to-main job is
# moving from staging-only plus a release-time gcrane copy to publishing both on merge.
resource "google_artifact_registry_repository_iam_member" "backend_github_actions_writer" {
  project    = google_artifact_registry_repository.backend.project
  location   = google_artifact_registry_repository.backend.location
  repository = google_artifact_registry_repository.backend.name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:github-actions@blog-gorman-club-root.iam.gserviceaccount.com"
}
