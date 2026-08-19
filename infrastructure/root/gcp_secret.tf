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
