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

# BlogRepository.List (services/backend/internal/repository/firestore/blog.go) pages the "blogs"
# collection ordered by createdAt, so it can walk a feed one page at a time instead of reading the
# whole collection on every call. Filtering the general feed on any of the three ways a post is
# readable is a Firestore OR query, which runs as one query per branch behind the scenes - and a
# query combining a filter with `ORDER BY createdAt` on a different field needs a composite index
# for that combination, one per branch. A profile feed's ownerId-only branch reuses the second of
# these.
resource "google_firestore_index" "blogs_by_visibility_and_created_at" {
  project    = var.gcp_project_id
  database   = google_firestore_database.database.name
  collection = "blogs"

  fields {
    field_path = "visibility"
    order      = "ASCENDING"
  }
  fields {
    field_path = "createdAt"
    order      = "DESCENDING"
  }
}

resource "google_firestore_index" "blogs_by_owner_and_created_at" {
  project    = var.gcp_project_id
  database   = google_firestore_database.database.name
  collection = "blogs"

  fields {
    field_path = "ownerId"
    order      = "ASCENDING"
  }
  fields {
    field_path = "createdAt"
    order      = "DESCENDING"
  }
}

resource "google_firestore_index" "blogs_by_allowed_user_and_created_at" {
  project    = var.gcp_project_id
  database   = google_firestore_database.database.name
  collection = "blogs"

  fields {
    field_path   = "allowedUserIds"
    array_config = "CONTAINS"
  }
  fields {
    field_path = "createdAt"
    order      = "DESCENDING"
  }
}

# Filtering the feed by tag (`GET /blogs?tag=...`) is an array-contains on "tags" ordered by
# createdAt, and needs its own composite index for the same reason the three above do. It replaces
# the readability OR rather than joining it - Firestore allows only one array-contains per query,
# and the OR branch already spends it on allowedUserIds - so a tagged feed is filtered on the tag
# here and on readability in Go, which is why there is no (visibility, tags) index beside these.
resource "google_firestore_index" "blogs_by_tag_and_created_at" {
  project    = var.gcp_project_id
  database   = google_firestore_database.database.name
  collection = "blogs"

  fields {
    field_path   = "tags"
    array_config = "CONTAINS"
  }
  fields {
    field_path = "createdAt"
    order      = "DESCENDING"
  }
}

# A profile feed narrowed to one tag (`?ownerId=...&tag=...`) filters on both at once, which is a
# third combination rather than either of the two above.
resource "google_firestore_index" "blogs_by_owner_tag_and_created_at" {
  project    = var.gcp_project_id
  database   = google_firestore_database.database.name
  collection = "blogs"

  fields {
    field_path = "ownerId"
    order      = "ASCENDING"
  }
  fields {
    field_path   = "tags"
    array_config = "CONTAINS"
  }
  fields {
    field_path = "createdAt"
    order      = "DESCENDING"
  }
}
