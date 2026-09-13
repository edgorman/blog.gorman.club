# One versioned static-build bucket per environment for the frontend, which is dropping its
# Artifact Registry image (see the frontend build issue tracked alongside this one): CI writes the
# built site here on merge and reads it back on deploy, and Cloudflare Pages serves the extracted
# files. Not public - nothing outside the pipeline needs to reach it directly.
resource "google_storage_bucket" "frontend" {
  project                     = var.gcp_project_id
  name                        = "frontend-${var.environment}"
  location                    = var.gcp_region
  force_destroy               = false
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
}

# CI needs to both write the build on merge and read it back on deploy; objectAdmin covers both
# without granting bucket-level control (ACLs, deletion) that neither step needs.
resource "google_storage_bucket_iam_member" "frontend_github_actions_writer" {
  bucket = google_storage_bucket.frontend.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:github-actions@blog-gorman-club-root.iam.gserviceaccount.com"
}
