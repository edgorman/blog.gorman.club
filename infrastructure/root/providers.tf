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

  # Blank: the first apply runs against local state (Makefile's `init` target) since the bucket
  # doesn't exist yet; state is migrated in afterward.
  backend "gcs" {
    bucket = ""
  }
}

provider "google" {
  project = var.gcp_provider_project_id
  region  = var.gcp_provider_region
  zone    = var.gcp_provider_zone
}

# Only writes the WORKLOAD_IDENTITY_PROVIDER / SERVICE_ACCOUNT Actions variables (github_cicd.tf);
# repository settings are managed separately by .github/settings.yml.
provider "github" {
  owner = var.github_repository_owner
  token = var.github_provider_token
}

# Manages Pages projects + custom domains (cloudflare_pages.tf); wrangler pushes builds into
# these same projects separately (see frontend-deploy).
provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
