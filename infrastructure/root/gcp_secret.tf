# Stores the GitHub PAT so CI can re-apply this module after the one-time manual bootstrap
# (read back via the gcp-secret-manager composite action as TF_VAR_github_provider_token).
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

# Same bootstrap pattern as github_provider_token above. Unlike it, these are never written to GitHub
# Actions secrets/variables - push-commit.yaml's frontend deploy job reads them from Secret Manager directly.
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
