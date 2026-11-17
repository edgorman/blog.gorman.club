# The tokens CI needs after the one-time manual bootstrap, each in Secret Manager and read by a
# workflow with its identity. The plan identity reads neither. PR plans use placeholder tokens (see
# terraform-pull-request), and secret_data is ignored so they show no change.

# The GitHub PAT: Terraform's github provider, release-please and Renovate. Read by deploy and renovate.
resource "google_secret_manager_secret" "github_provider_token" {
  project   = var.gcp_provider_project_id
  secret_id = "github_provider_token"

  replication {
    auto {}
  }
}

# Set at bootstrap. To rotate it, add a new version in Secret Manager: workflows read `latest`.
resource "google_secret_manager_secret_version" "github_provider_token_v1" {
  secret      = google_secret_manager_secret.github_provider_token.id
  secret_data = var.github_provider_token

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_iam_member" "github_provider_token_accessors" {
  for_each = {
    deploy   = google_service_account.github_deploy.email
    renovate = google_service_account.github_renovate.email
  }

  project   = var.gcp_provider_project_id
  secret_id = google_secret_manager_secret.github_provider_token.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${each.value}"
}

# The Cloudflare API token: Terraform's cloudflare provider and wrangler. Read by deploy and promote.
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

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_iam_member" "cloudflare_api_token_accessors" {
  for_each = {
    deploy  = google_service_account.github_deploy.email
    promote = google_service_account.github_promote.email
  }

  project   = var.gcp_provider_project_id
  secret_id = google_secret_manager_secret.cloudflare_api_token.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${each.value}"
}
