resource "google_project_service" "firestore" {
  project = var.gcp_project_id
  service = "firestore.googleapis.com"
}

# Vestigial: Firebase was needed when the security rules and Firebase Auth were in use. Sign-in
# is now Google Identity Services verified directly by the backend (services/backend/auth.go),
# and Firestore itself needs none of this. Left in place because Firebase cannot be un-added from
# a project, so removing these resources only drops them from state; worth a deliberate cleanup
# rather than a drive-by one.
resource "google_project_service" "firebase" {
  project = var.gcp_project_id
  service = "firebase.googleapis.com"
}

# Adds Firebase to the GCP project. Beta-only. Uses the quota-overriding provider alias (see
# providers.tf).
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

# No Firestore security rules are deployed, deliberately. The backend is the only client of this
# database and it authenticates as a service account, which bypasses rules anyway; access is
# decided in services/backend (canRead, requireOwnedBlog). With no ruleset released, the Firebase
# client SDKs cannot reach the database at all, which is exactly the intent - a browser talks to
# the API, never to Firestore.
