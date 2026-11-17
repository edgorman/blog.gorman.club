# The root project is created by hand (nothing exists yet to create it with) and imported here;
# staging/prod are created by Terraform.
resource "google_project" "root_project" {
  name       = "${var.gcp_project_prefix} root"
  project_id = var.gcp_provider_project_id

  lifecycle {
    ignore_changes = [billing_account]
  }
}

import {
  id = var.gcp_provider_project_id
  to = google_project.root_project
}

resource "google_project" "env_projects" {
  for_each        = toset(var.gcp_projects)
  name            = "${var.gcp_project_prefix} ${each.value}"
  project_id      = "${var.gcp_project_prefix}-${each.value}"
  billing_account = google_project.root_project.billing_account
}

locals {
  all_projects = merge(
    { "root" = google_project.root_project },
    google_project.env_projects
  )
}

# Baseline APIs shared by every project; app-specific ones are enabled per-environment.
resource "google_project_service" "project_services" {
  for_each = {
    for pair in setproduct(keys(local.all_projects), [
      "storage.googleapis.com",
      "secretmanager.googleapis.com",
      "iam.googleapis.com",
      "cloudresourcemanager.googleapis.com",
      "iamcredentials.googleapis.com",
      "sts.googleapis.com",
      ]) : "${pair[0]}-${pair[1]}" => {
      project_id = local.all_projects[pair[0]].project_id
      service    = pair[1]
    }
  }
  project = each.value.project_id
  service = each.value.service
}

resource "google_storage_bucket" "gcp_project_terraform_states" {
  depends_on = [
    google_project_service.project_services,
    google_project.root_project,
    google_project.env_projects
  ]

  for_each                    = local.all_projects
  project                     = var.gcp_provider_project_id
  name                        = "${each.value.project_id}-terraform-state"
  location                    = var.gcp_provider_region
  force_destroy               = false
  public_access_prevention    = "enforced"
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }
}
