resource "google_project_service" "firestore" {
  project = var.gcp_project_id
  service = "firestore.googleapis.com"
}

resource "google_project_service" "firebaserules" {
  project = var.gcp_project_id
  service = "firebaserules.googleapis.com"
}

# Needed to add Firebase to the project (google_firebase_project below) - the Firebase Rules API
# rejects requests with SERVICE_DISABLED until the project has Firebase added, even once
# firebaserules.googleapis.com itself is enabled.
resource "google_project_service" "firebase" {
  project = var.gcp_project_id
  service = "firebase.googleapis.com"
}

# Newly enabled APIs take a while to propagate; using them immediately in the same apply
# intermittently fails with 403s. Each API gets its own time_sleep: once a time_sleep resource
# exists in state, adding a new dependency to it doesn't make it wait again (depends_on changes
# don't force recreation), so a shared sleep silently stops covering APIs enabled after the first
# apply that created it - one bit us going from firestore/firebaserules to firebase.googleapis.com.
resource "time_sleep" "firestore_apis" {
  depends_on = [google_project_service.firestore, google_project_service.firebaserules]

  create_duration = "60s"
}

resource "time_sleep" "firebase_api" {
  depends_on = [google_project_service.firebase]

  create_duration = "60s"
}

# Beta-only resource: adds Firebase to the project. Required for the Firebase Rules API to work
# at all (see comment on google_project_service.firebase above) - without it, ruleset/release
# creation 403s with "Firebase Rules API has not been used ... or is disabled" regardless of how
# long you wait after enabling the raw API.
resource "google_firebase_project" "default" {
  provider = google-beta

  project    = var.gcp_project_id
  depends_on = [time_sleep.firebase_api]
}

resource "google_firestore_database" "database" {
  depends_on = [time_sleep.firestore_apis, google_firebase_project.default]

  project     = var.gcp_project_id
  name        = "(default)"
  location_id = var.gcp_region
  type        = "FIRESTORE_NATIVE"
}

# Security rules for the `users` and `blogs` collections (see firestore.rules), deployed as a
# Firebase Rules ruleset. Content changes force a new ruleset (rulesets are immutable), so the
# release below is updated to point at it in the same apply.
resource "google_firebaserules_ruleset" "firestore" {
  depends_on = [google_firebase_project.default]

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
  depends_on = [google_firestore_database.database, google_firebase_project.default]

  project      = var.gcp_project_id
  name         = "cloud.firestore"
  ruleset_name = "projects/${var.gcp_project_id}/rulesets/${google_firebaserules_ruleset.firestore.name}"
}
