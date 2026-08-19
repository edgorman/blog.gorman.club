# Stores the GitHub PAT so CI can re-apply this root module on every future
# push to main (not just the one-time manual bootstrap): the pull-request
# and push-commit workflows read it back out via the gcp-secret-manager
# composite action and pass it in as TF_VAR_github_provider_token, which the
# `github` provider in providers.tf needs to manage the two Actions
# variables in github_cicd.tf.
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

# Same bootstrap-then-Secret-Manager pattern as github_provider_token above,
# for the two Cloudflare values cloudflare_cicd.tf writes into GitHub
# Actions. cloudflare_account_id isn't sensitive, but it still needs a
# source on every apply since it has no default, so it goes through Secret
# Manager the same way.
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
