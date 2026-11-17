# LEGACY (decision 38): the one service account the workflows used before the plan / deploy /
# promote / renovate split in github_cicd.tf. It is kept, unchanged, for one release so that the
# old workflows (and any run in flight) still authenticate between this root's first apply and the
# merge that switches the workflows over. It is the credential the split exists to retire: any run
# in this repository can assume it, and it holds apply roles on every project. A follow-up PR,
# opened once the first push-commit.yaml run on main has succeeded as the deploy identity, deletes
# this file, its Secret Manager bindings in gcp_secret.tf and the SERVICE_ACCOUNT variable.
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

# The backend repository's writer binding in env/artifact_registry.tf needs
# artifactregistry.repositories.setIamPolicy, which roles/editor excludes. repoAdmin isn't enough
# for this despite the name - it covers repository content administration (deleting images,
# managing tags, cleanup policies) but not IAM policy on the repository itself; only
# artifactregistry.admin reaches that, confirmed against staging still failing after repoAdmin
# was granted and propagation delay was ruled out.
resource "google_project_iam_member" "github_actions_artifact_registry_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/artifactregistry.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# The frontend bucket's writer binding in env/storage.tf needs storage.buckets.setIamPolicy,
# which roles/editor excludes the same way it excludes setIamPolicy on the resources above.
# storage.buckets.create itself needs no extra grant - that part is already covered by editor.
resource "google_project_iam_member" "github_actions_storage_admin" {
  for_each = local.all_projects

  project = each.value.project_id
  role    = "roles/storage.admin"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

resource "google_service_account_iam_member" "github_actions_wic" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/${var.github_repository_owner}/${var.github_repository_name}"
}

# The legacy service account's secret access, and the Cloudflare account id secret only it reads: the
# id is now a plain variable (CLOUDFLARE_ACCOUNT_ID), so the new workflows read no secret for it.
resource "google_secret_manager_secret_iam_member" "github_actions_secret_accessor" {
  project   = var.gcp_provider_project_id
  secret_id = google_secret_manager_secret.github_provider_token.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.github_actions.email}"
}

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

resource "google_secret_manager_secret_iam_member" "github_actions_cloudflare_api_token_accessor" {
  project   = var.gcp_provider_project_id
  secret_id = google_secret_manager_secret.cloudflare_api_token.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.github_actions.email}"
}
