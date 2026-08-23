# Registers a Firebase Web App under this project, giving the frontend the (non-secret) client
# config it needs to initialize the Firebase JS SDK for direct Firestore reads and auth.
resource "google_firebase_web_app" "frontend" {
  provider = google-beta.firebase

  project      = var.gcp_project_id
  display_name = "frontend-${var.environment}"

  depends_on = [google_firebase_project.default]
}

data "google_firebase_web_app_config" "frontend" {
  provider = google-beta.firebase

  project    = var.gcp_project_id
  web_app_id = google_firebase_web_app.frontend.app_id
}

resource "google_project_service" "identitytoolkit" {
  project = var.gcp_project_id
  service = "identitytoolkit.googleapis.com"
}

# Anonymous sign-in only, for now: enough to satisfy firestore.rules' `request.auth != null` for
# reads (including public blogs) without a real account system. A persistent identity for the
# blog's actual author (e.g. Google sign-in) is a follow-up product decision, not implemented here.
resource "google_identity_platform_config" "default" {
  provider = google.firebase

  project = var.gcp_project_id

  sign_in {
    anonymous {
      enabled = true
    }
  }

  depends_on = [google_project_service.identitytoolkit, google_firebase_project.default]
}
