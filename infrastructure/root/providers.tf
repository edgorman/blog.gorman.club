terraform {
  required_version = ">= 1.9.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    github = {
      source  = "integrations/github"
      version = "~> 6.0"
    }
  }

  # Bucket is intentionally blank here — the very first apply runs against
  # local state (see the Makefile's `init` target) since this bucket doesn't
  # exist yet. Once it does, state is migrated in via
  # `terraform init -migrate-state -backend-config=../config/root/terraform.tfbackend`.
  backend "gcs" {
    bucket = ""
  }
}

provider "google" {
  project = var.gcp_provider_project_id
  region  = var.gcp_provider_region
  zone    = var.gcp_provider_zone
}

# Only used to write the WORKLOAD_IDENTITY_PROVIDER / SERVICE_ACCOUNT GitHub
# Actions variables (github_cicd.tf) — repository-level settings, branch
# protection, and rulesets are managed separately by .github/settings.yml
# via the repository-settings/app, not by Terraform.
provider "github" {
  owner = var.github_repository_owner
  token = var.github_provider_token
}
