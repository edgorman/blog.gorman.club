# Stores the GitHub PAT so CI can re-apply this module after the one-time manual bootstrap.
resource "google_secret_manager_secret" "github_provider_token" {
  project   = var.gcp_provider_project_id
  secret_id = "github_provider_token"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "github_provider_token_v1" {
  secret      = google_secret_manager_secret.github_provider_token.id
  secret_data = var.github_provider_token
}

resource "google_secret_manager_secret_iam_member" "github_actions_secret_accessor" {
  project   = var.gcp_provider_project_id
  secret_id = google_secret_manager_secret.github_provider_token.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.github_actions.email}"
}

# Same bootstrap pattern, except workflows read these from Secret Manager directly rather than
# from a GitHub Actions secret.
resource "google_secret_manager_secret" "cloudflare_account_id" {
  project   = var.gcp_provider_project_id
  secret_id = "cloudflare_account_id"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "cloudflare_account_id_v1" {
  secret      = google_secret_manager_secret.cloudflare_account_id.id
  secret_data = var.cloudflare_account_id
}

resource "google_secret_manager_secret_iam_member" "github_actions_cloudflare_account_id_accessor" {
  project   = var.gcp_provider_project_id
  secret_id = google_secret_manager_secret.cloudflare_account_id.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_secret_manager_secret" "cloudflare_api_token" {
  project   = var.gcp_provider_project_id
  secret_id = "cloudflare_api_token"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "cloudflare_api_token_v1" {
  secret      = google_secret_manager_secret.cloudflare_api_token.id
  secret_data = var.cloudflare_api_token
}

resource "google_secret_manager_secret_iam_member" "github_actions_cloudflare_api_token_accessor" {
  project   = var.gcp_provider_project_id
  secret_id = google_secret_manager_secret.cloudflare_api_token.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.github_actions.email}"
}

# The billing account id, so infrastructure/env can create a budget under it without the id being
# written into a tfvars file in a public repository. Unlike the secrets above it is not supplied by
# hand at bootstrap: root already holds it, read off the project it imported, so there is nothing
# for an operator to paste in and nothing to keep in step by hand.
resource "google_secret_manager_secret" "gcp_billing_account" {
  project   = var.gcp_provider_project_id
  secret_id = "gcp_billing_account"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "gcp_billing_account_v1" {
  secret = google_secret_manager_secret.gcp_billing_account.id
  # Bare id (e.g. 012345-6789AB-CDEF01), which is the form google_billing_budget takes.
  secret_data = trimprefix(google_project.root_project.billing_account, "billingAccounts/")
}

resource "google_secret_manager_secret_iam_member" "github_actions_gcp_billing_account_accessor" {
  project   = var.gcp_provider_project_id
  secret_id = google_secret_manager_secret.gcp_billing_account.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.github_actions.email}"
}
