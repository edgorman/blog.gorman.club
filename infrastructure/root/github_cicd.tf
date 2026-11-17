# Four identities for GitHub Actions to authenticate to GCP as, without a long-lived JSON key, so
# that a workflow edited on a pull request branch holds nothing that can write anywhere:
#
#   plan    read-only; assumable by any run in this repository, which is what the PR checks use.
#   deploy  push-commit.yaml as it exists on main: root and staging applies, builds, staging
#           deploys, and reading the tokens release-please and Cloudflare need.
#   promote promote-release.yaml and apply-infrastructure-prod.yaml as they exist on main: prod's
#           apply and deploy. Only prod's project, state bucket and the Cloudflare token.
#   renovate renovate.yaml as it exists on main: it reads the GitHub PAT and nothing else, because
#           it runs a third-party action.
#
# The apply identities are bound on the workflow file and the ref it is loaded from (the
# job_workflow_ref claim, e.g. <repo>/.github/workflows/push-commit.yaml@refs/heads/main), which a
# pull request cannot forge: a PR's runs carry refs/pull/<n>/merge. Nothing here needs a GitHub
# Environment, so it holds on any plan, private repositories included.
#
# Limits. Root owns IAM for every project, so the deploy identity can change IAM on prod too: the
# split keeps prod's deploy credentials away from everything but main's workflows, not from a
# change merged to main. And on a repository without branch protection, anything that can push to
# main can change those workflows.
resource "google_service_account" "github_plan" {
  project      = var.gcp_provider_project_id
  account_id   = "github-plan"
  display_name = "GitHub Actions - plan (read-only)"
}

resource "google_service_account" "github_deploy" {
  project      = var.gcp_provider_project_id
  account_id   = "github-deploy"
  display_name = "GitHub Actions - deploy (main, push-commit.yaml)"
}

resource "google_service_account" "github_renovate" {
  project      = var.gcp_provider_project_id
  account_id   = "github-renovate"
  display_name = "GitHub Actions - Renovate (main)"
}

resource "google_service_account" "github_promote" {
  project      = var.gcp_provider_project_id
  account_id   = "github-promote"
  display_name = "GitHub Actions - promote to prod (main)"
}

locals {
  repository = "${var.github_repository_owner}/${var.github_repository_name}"

  # What a plan reads. viewer covers reading resources and state; securityReviewer adds the IAM
  # policies Terraform refreshes; serviceUsageConsumer is for env/providers.tf's user_project_override.
  plan_roles = [
    "roles/viewer",
    "roles/iam.securityReviewer",
    "roles/serviceusage.serviceUsageConsumer",
  ]

  # What an apply manages with, and why each is needed beyond roles/editor:
  #   projectIamAdmin     editor excludes IAM management, which this root's own applies need.
  #   run.admin           the public-invoker binding in env/cloud_run.tf needs run.services.setIamPolicy.
  #   datastore.admin     google_firestore_database needs datastore.databases.create.
  #   firebase.admin      google_firebase_project needs firebase.projects.update.
  #   serviceAccountAdmin backend_runtime_actas in env/cloud_run.tf needs iam.serviceAccounts.setIamPolicy.
  #   serviceUsageConsumer env/providers.tf's user_project_override redirects API quota off the root
  #                       project these identities live in.
  #   artifactregistry.admin / storage.admin  the repository and bucket bindings in artifact_stores.tf
  #                       need setIamPolicy, which editor excludes (repoAdmin is not enough).
  apply_roles = [
    "roles/editor",
    "roles/resourcemanager.projectIamAdmin",
    "roles/run.admin",
    "roles/datastore.admin",
    "roles/firebase.admin",
    "roles/iam.serviceAccountAdmin",
    "roles/serviceusage.serviceUsageConsumer",
    "roles/artifactregistry.admin",
    "roles/storage.admin",
  ]

  # Granted per project, never org- or folder-wide.
  plan_grants = {
    for pair in setproduct(keys(local.all_projects), local.plan_roles) :
    "${pair[0]}-${pair[1]}" => { project = local.all_projects[pair[0]].project_id, role = pair[1] }
  }
  deploy_grants = {
    for pair in setproduct(keys(local.all_projects), local.apply_roles) :
    "${pair[0]}-${pair[1]}" => { project = local.all_projects[pair[0]].project_id, role = pair[1] }
  }
  promote_grants = {
    for role in local.apply_roles :
    "prod-${role}" => { project = local.all_projects["prod"].project_id, role = role }
  }
}

resource "google_project_iam_member" "github_plan" {
  for_each = local.plan_grants

  project = each.value.project
  role    = each.value.role
  member  = "serviceAccount:${google_service_account.github_plan.email}"
}

resource "google_project_iam_member" "github_deploy" {
  for_each = local.deploy_grants

  project = each.value.project
  role    = each.value.role
  member  = "serviceAccount:${google_service_account.github_deploy.email}"
}

resource "google_project_iam_member" "github_promote" {
  for_each = local.promote_grants

  project = each.value.project
  role    = each.value.role
  member  = "serviceAccount:${google_service_account.github_promote.email}"
}

# Prod's Terraform state lives in a bucket in the root project, so the promote identity needs that
# one bucket and nothing else there.
resource "google_storage_bucket_iam_member" "github_promote_prod_state" {
  bucket = google_storage_bucket.gcp_project_terraform_states["prod"].name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.github_promote.email}"
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
    "google.subject"             = "assertion.sub"
    "attribute.repository"       = "assertion.repository"
    "attribute.actor"            = "assertion.actor"
    "attribute.job_workflow_ref" = "assertion.job_workflow_ref"
  }

  # Restricts to OIDC tokens asserting this exact repository.
  attribute_condition = "assertion.repository == '${var.github_repository_owner}/${var.github_repository_name}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

# Plan: any run in this repository.
resource "google_service_account_iam_member" "github_plan_wif" {
  service_account_id = google_service_account.github_plan.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/${local.repository}"
}

# Deploy, promote and Renovate: only the named workflow files as they exist on main.
resource "google_service_account_iam_member" "github_renovate_wif" {
  service_account_id = google_service_account.github_renovate.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.job_workflow_ref/${local.repository}/.github/workflows/renovate.yaml@refs/heads/main"
}

resource "google_service_account_iam_member" "github_deploy_wif" {
  for_each = toset(["push-commit.yaml"])

  service_account_id = google_service_account.github_deploy.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.job_workflow_ref/${local.repository}/.github/workflows/${each.key}@refs/heads/main"
}

resource "google_service_account_iam_member" "github_promote_wif" {
  for_each = toset(["promote-release.yaml", "apply-infrastructure-prod.yaml"])

  service_account_id = google_service_account.github_promote.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.job_workflow_ref/${local.repository}/.github/workflows/${each.key}@refs/heads/main"
}

# Wired into Actions repository variables so workflows never hardcode them. Their presence is also
# what turns on every infrastructure job: until this root's first apply writes them, the workflows
# run only the PR checks (see the `changed` jobs).
locals {
  actions_variables = {
    WORKLOAD_IDENTITY_PROVIDER = google_iam_workload_identity_pool_provider.github_provider.name
    PLAN_SERVICE_ACCOUNT       = google_service_account.github_plan.email
    DEPLOY_SERVICE_ACCOUNT     = google_service_account.github_deploy.email
    PROMOTE_SERVICE_ACCOUNT    = google_service_account.github_promote.email
    RENOVATE_SERVICE_ACCOUNT   = google_service_account.github_renovate.email
    # Not a secret: it appears in the Cloudflare dashboard's URLs. frontend-deploy passes it to wrangler.
    CLOUDFLARE_ACCOUNT_ID = var.cloudflare_account_id
    # One client ID for every environment: passed to infrastructure/env as TF_VAR_google_client_id
    # and baked into the frontend build as VITE_GOOGLE_CLIENT_ID.
    GOOGLE_CLIENT_ID = var.google_client_id
  }
}

resource "github_actions_variable" "variables" {
  for_each = local.actions_variables

  repository    = var.github_repository_name
  variable_name = each.key
  value         = each.value
}

# These were separate resources before the variables became one map; moving them keeps the
# apply from deleting and recreating the variables the live workflows read.
moved {
  from = github_actions_variable.workload_identity_provider
  to   = github_actions_variable.variables["WORKLOAD_IDENTITY_PROVIDER"]
}

moved {
  from = github_actions_variable.google_client_id
  to   = github_actions_variable.variables["GOOGLE_CLIENT_ID"]
}
