resource "google_project_service" "firestore" {
  project = var.gcp_project_id
  service = "firestore.googleapis.com"
}

resource "google_project_service" "firebaserules" {
  project = var.gcp_project_id
  service = "firebaserules.googleapis.com"
}

# Adding Firebase to the project (google_firebase_project below) is a prerequisite for the
# Firebase Rules API that serves the security rules.
resource "google_project_service" "firebase" {
  project = var.gcp_project_id
  service = "firebase.googleapis.com"
}

# Adds Firebase to the GCP project. Beta-only, and required before any Firebase Rules resource
# will work. Uses the quota-overriding provider alias (see providers.tf).
resource "google_firebase_project" "default" {
  provider = google-beta.firebase

  depends_on = [google_project_service.firebase]

  project = var.gcp_project_id
}

resource "google_firestore_database" "database" {
  provider = google.firebase

  depends_on = [google_project_service.firestore, google_firebase_project.default]

  project     = var.gcp_project_id
  name        = "(default)"
  location_id = var.gcp_region
  type        = "FIRESTORE_NATIVE"
}

# Security rules for the `users` and `blogs` collections (see firestore.rules). Rulesets are
# immutable, so changing the file creates a new one and the release below is repointed at it.
resource "google_firebaserules_ruleset" "firestore" {
  provider = google.firebase

  depends_on = [google_project_service.firebaserules, google_firebase_project.default]

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
  provider = google.firebase

  depends_on = [google_firestore_database.database]

  project      = var.gcp_project_id
  name         = "cloud.firestore"
  ruleset_name = "projects/${var.gcp_project_id}/rulesets/${google_firebaserules_ruleset.firestore.name}"
}
