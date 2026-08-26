# WIF identity GitHub Actions uses to authenticate to GCP without a long-lived JSON key.

resource "google_service_account" "github_actions" {
  project      = var.gcp_provider_project_id
  account_id   = "github-actions"
  display_name = "GitHub Actions Service Account"
}

# Granted per-project on root and every environment, never org- or folder-wide.
resource "google_project_iam_member" "github_actions_editor" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/editor"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# roles/editor excludes IAM management, which this root's own applies need.
resource "google_project_iam_member" "github_actions_iam_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/resourcemanager.projectIamAdmin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# The public-invoker binding in env/cloud_run.tf needs run.services.setIamPolicy.
resource "google_project_iam_member" "github_actions_run_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# google_firestore_database needs datastore.databases.create, which roles/editor omits.
resource "google_project_iam_member" "github_actions_datastore_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/datastore.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# google_firebase_project needs firebase.projects.update, which roles/editor omits.
resource "google_project_iam_member" "github_actions_firebase_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/firebase.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# backend_runtime_actas in env/cloud_run.tf needs iam.serviceAccounts.setIamPolicy, which is
# distinct from actAs itself and covered by none of the roles above.
resource "google_project_iam_member" "github_actions_service_account_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/iam.serviceAccountAdmin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# Required by env/providers.tf's user_project_override, which redirects API quota off the root
# project this service account lives in.
resource "google_project_iam_member" "github_actions_service_usage_consumer" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
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

  # Restricts to OIDC tokens asserting this exact repository.
  attribute_condition = "assertion.repository == '${var.github_repository_owner}/${var.github_repository_name}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "github_actions_wic" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/${var.github_repository_owner}/${var.github_repository_name}"
}

# Wired into Actions repository variables so workflows never hardcode them.
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

# One client ID for every environment: passed to infrastructure/env as TF_VAR_google_client_id and
# baked into the frontend build as VITE_GOOGLE_CLIENT_ID.
resource "github_actions_variable" "google_client_id" {
  repository    = var.github_repository_name
  variable_name = "GOOGLE_CLIENT_ID"
  value         = var.google_client_id
}
