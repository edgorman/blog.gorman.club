terraform {
  required_version = "1.15.8"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.45.0"
    }
    github = {
      source  = "integrations/github"
      version = "6.13.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "5.23.0"
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

# Manages the Pages projects + custom domains (cloudflare_pages.tf) that the
# services-frontend deploy jobs (frontend-deploy composite action) push
# builds into via wrangler. Deploy credentials and this provider's
# api_token both ultimately come from the same cloudflare_api_token secret
# (see gcp_secret.tf) — passed by hand at bootstrap, read from Secret
# Manager on every apply after that.
provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
