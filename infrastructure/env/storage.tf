# One versioned static-build bucket per environment for the frontend, which is dropping its
# Artifact Registry image (see the frontend build issue tracked alongside this one): CI writes the
# built site here on merge and reads it back on deploy, and Cloudflare Pages serves the extracted
# files. Not public - nothing outside the pipeline needs to reach it directly.
#
# Named after the project rather than just the environment because bucket names are global across
# all of GCS, not scoped to a project like most other resources here - the same reason the
# Terraform state buckets in infrastructure/root are prefixed the same way.
resource "google_storage_bucket" "frontend" {
  project                     = var.gcp_project_id
  name                        = "${var.gcp_project_id}-frontend"
  location                    = var.gcp_region
  force_destroy               = false
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  # Every merge publishes a commit-SHA folder here in both buckets alike (see Staging Deployments
  # in CLAUDE.md), so like the backend registry, this fills up from merges whether or not the
  # environment it lives in is promoted to often. Unlike the registry's keep-count policy, GCS
  # lifecycle rules only condition on object age - there is no "keep the N most recent folders"
  # equivalent - so this expires anything older than frontend_retention_days outright. See the
  # Artifact Stores section of CLAUDE.md for how that number was chosen and, since an age-based
  # rule cannot single out "the folder currently deployed to prod" the way the registry's keep-count
  # can single out recent versions, what has to stay true operationally for it to never be caught by
  # this rule.
  lifecycle_rule {
    condition {
      age = var.frontend_retention_days
    }

    action {
      type = "Delete"
    }
  }
}

# CI needs to both write the build on merge and read it back on deploy; objectAdmin covers both
# without granting bucket-level control (ACLs, deletion) that neither step needs.
resource "google_storage_bucket_iam_member" "frontend_github_actions_writer" {
  bucket = google_storage_bucket.frontend.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:github-actions@blog-gorman-club-root.iam.gserviceaccount.com"
}
