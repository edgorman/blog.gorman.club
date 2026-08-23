# WIF identity GitHub Actions uses to authenticate to GCP without a long-lived JSON key.
# Repository settings themselves are managed separately by .github/settings.yml.

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

# roles/editor excludes IAM management; this lets CI manage each project's own IAM policy (needed since
# this root's own applies manage IAM bindings). Doesn't cover resource-level policies - granted below.
resource "google_project_iam_member" "github_actions_iam_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/resourcemanager.projectIamAdmin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# Cloud Run's public-invoker binding (infrastructure/cloud_run.tf) needs run.services.setIamPolicy,
# covered by neither role above.
resource "google_project_iam_member" "github_actions_run_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# google_firestore_database (infrastructure/env/firestore.tf) needs datastore.databases.create,
# which roles/editor doesn't include.
resource "google_project_iam_member" "github_actions_datastore_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/datastore.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# google_firebase_project (infrastructure/env/firestore.tf) needs firebase.projects.update to add
# Firebase to the project, which roles/editor doesn't include.
resource "google_project_iam_member" "github_actions_firebase_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/firebase.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# google_firebaserules_ruleset/release (infrastructure/env/firestore.tf) need
# firebaserules.rulesets.create and firebaserules.releases.create, covered by neither role above.
resource "google_project_iam_member" "github_actions_firebaserules_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/firebaserules.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# This service account lives in the root project, so Firebase/Firestore calls would otherwise bill
# quota to root rather than the project being modified. infrastructure/env uses the provider's
# user_project_override to redirect that, which requires serviceusage.services.use on the target.
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
