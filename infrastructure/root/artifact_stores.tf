# Each environment's artifact stores: the registry for the backend and worker images and the
# frontend build bucket. They live here rather than in infrastructure/env because every merge
# publishes into every environment's stores (see Staging Deployments in `.github/AGENTS.md`), while
# env Terraform only reaches prod on promotion. Root applies ahead of staging on every merge it
# affects, and at bootstrap, so prod's stores exist before anything is pushed to them (decision 34).
# A new store any merge writes to belongs here too, for the same reason.

resource "google_project_service" "artifact_registry" {
  for_each = google_project.env_projects

  project = each.value.project_id
  service = "artifactregistry.googleapis.com"
}

# Every merge writes a commit-SHA image here in both projects alike, so each registry grows with
# merges whether or not its environment is deployed often. The cleanup_policies below are Artifact
# Registry's standard "keep only the N most recent": the KEEP policies pin the most recent
# backend_registry_keep_count versions regardless of age (plus every release-tagged image), and the
# DELETE policy removes anything a KEEP policy didn't pin. See the Artifact Stores section of
# `infrastructure/AGENTS.md` for how the count was chosen and what it does not guarantee (there is
# no policy condition for "a Cloud Run revision still references this").
#
# Ships with cleanup_policy_dry_run enabled: the policy evaluates and logs what it would delete
# without deleting anything, so the first real cleanup pass can be reviewed in Cloud Logging before
# a follow-up change flips this to false.
resource "google_artifact_registry_repository" "backend" {
  for_each   = google_project.env_projects
  depends_on = [google_project_service.artifact_registry]

  project       = each.value.project_id
  location      = var.gcp_provider_region
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
  # on top of its commit-SHA tag, so a rollback target never falls out of the window above (#279).
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

# Granted on the repository itself rather than relying on the project-wide roles in
# github_cicd.tf: a grant lives beside the resource it applies to.
resource "google_artifact_registry_repository_iam_member" "backend_github_actions_writer" {
  for_each = google_artifact_registry_repository.backend

  project    = each.value.project
  location   = each.value.location
  repository = each.value.name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.github_actions.email}"
}

# One folder per commit SHA (that is its versioning; the bucket itself has none): CI writes the
# built site here on merge and reads it back on deploy, and Cloudflare Pages serves the extracted
# files. Not public. Named after the project because bucket names are global across GCS.
resource "google_storage_bucket" "frontend" {
  for_each   = google_project.env_projects
  depends_on = [google_project_service.project_services]

  project                     = each.value.project_id
  name                        = "${each.value.project_id}-frontend"
  location                    = var.gcp_provider_region
  force_destroy               = false
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  # GCS lifecycle rules only condition on age - there is no "keep the N most recent folders" - so
  # this expires anything older than frontend_retention_days, including a live folder: promote, or
  # raise the window, before the folder prod runs ages out (see infrastructure/AGENTS.md).
  lifecycle_rule {
    condition {
      age = var.frontend_retention_days
    }

    action {
      type = "Delete"
    }
  }
}

# CI writes the build on merge and reads it back on deploy; objectAdmin covers both without
# bucket-level control (ACLs, deletion) that neither step needs.
resource "google_storage_bucket_iam_member" "frontend_github_actions_writer" {
  for_each = google_storage_bucket.frontend

  bucket = each.value.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.github_actions.email}"
}
