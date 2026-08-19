# Identity used by GitHub Actions to authenticate to GCP, plus the WIF
# plumbing that lets it do so without a long-lived JSON key. This file does
# NOT manage the GitHub repository itself (settings, branches, rulesets) —
# that's handled declaratively by .github/settings.yml via the
# repository-settings/app instead.

resource "google_service_account" "github_actions" {
  project      = var.gcp_provider_project_id
  account_id   = "github-actions"
  display_name = "GitHub Actions Service Account"
}

# Granted per-project, individually, on root + every environment — never a
# broad org-level or folder-level grant.
resource "google_project_iam_member" "github_actions_editor" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/editor"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# roles/editor deliberately excludes IAM policy management. This is required
# on top of it so CI can manage per-resource IAM policies it owns — e.g.
# Cloud Run service-to-service invocation bindings.
resource "google_project_iam_member" "github_actions_iam_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/resourcemanager.projectIamAdmin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_iam_workload_identity_pool" "github_pool" {
  project                   = var.gcp_provider_project_id
  workload_identity_pool_id = "github-pool"
  display_name              = "GitHub Pool"
}

resource "google_iam_workload_identity_pool_provider" "github_provider" {
  project                            = var.gcp_provider_project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider"
  display_name                       = "GitHub Provider"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.actor"      = "assertion.actor"
  }

  # Restricts which OIDC tokens can assume the service account below to
  # those asserting this exact repository.
  attribute_condition = "assertion.repository == '${var.github_repository_owner}/${var.github_repository_name}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "github_actions_wic" {
  service_account_id = google_service_account.github_actions.name
  role                = "roles/iam.workloadIdentityUser"
  member              = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/${var.github_repository_owner}/${var.github_repository_name}"
}

# Wire the resulting provider path and service account email into GitHub
# Actions repository variables so workflows never hardcode them.
resource "github_actions_variable" "workload_identity_provider" {
  repository    = var.github_repository_name
  variable_name = "WORKLOAD_IDENTITY_PROVIDER"
  value         = google_iam_workload_identity_pool_provider.github_provider.name
}

resource "github_actions_variable" "service_account" {
  repository    = var.github_repository_name
  variable_name = "SERVICE_ACCOUNT"
  value         = google_service_account.github_actions.email
}
