resource "google_project_service" "firestore" {
  project = var.gcp_project_id
  service = "firestore.googleapis.com"
}

# Vestigial: nothing needs Firebase now that the backend verifies sign-in itself, but Firebase
# cannot be un-added from a project, so removing these would only drop them from state.
resource "google_project_service" "firebase" {
  project = var.gcp_project_id
  service = "firebase.googleapis.com"
}

# Beta-only, and uses the quota-overriding provider alias (see providers.tf).
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

# No security rules are deployed deliberately: the backend is the only client and decides access
# itself, and with no ruleset released the client SDKs cannot reach the database at all.
