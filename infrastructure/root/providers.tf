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

  # Blank because the bucket doesn't exist on the first apply; see the Makefile's `init` target.
  backend "gcs" {
    bucket = ""
  }
}

provider "google" {
  project = var.gcp_provider_project_id
  region  = var.gcp_provider_region
  zone    = var.gcp_provider_zone
}

# Only writes the Actions variables in github_cicd.tf; repository settings live in
# .github/settings.yml.
provider "github" {
  owner = var.github_repository_owner
  token = var.github_provider_token
}

# Manages Pages projects and domains; wrangler pushes builds into them separately.
provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
