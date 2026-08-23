resource "google_project_service" "firestore" {
  project = var.gcp_project_id
  service = "firestore.googleapis.com"
}

resource "google_project_service" "firebaserules" {
  project = var.gcp_project_id
  service = "firebaserules.googleapis.com"
}

resource "google_firestore_database" "database" {
  depends_on = [google_project_service.firestore]

  project     = var.gcp_project_id
  name        = "(default)"
  location_id = var.gcp_region
  type        = "FIRESTORE_NATIVE"
}

# Security rules for the `users` and `blogs` collections (see firestore.rules), deployed as a
# Firebase Rules ruleset. Content changes force a new ruleset (rulesets are immutable), so the
# release below is updated to point at it in the same apply.
resource "google_firebaserules_ruleset" "firestore" {
  depends_on = [google_project_service.firebaserules]

  project = var.gcp_project_id

  source {
    files {
      name    = "firestore.rules"
      content = file("${path.module}/firestore.rules")
    }
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "google_firebaserules_release" "firestore" {
  depends_on = [google_firestore_database.database]

  project      = var.gcp_project_id
  name         = "cloud.firestore"
  ruleset_name = "projects/${var.gcp_project_id}/rulesets/${google_firebaserules_ruleset.firestore.name}"
}
