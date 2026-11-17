terraform {
  required_version = "1.17.2"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "8.8.1"
    }
    github = {
      source  = "integrations/github"
      version = "6.13.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "5.28.1"
    }
  }

  # Set with -backend-config; the Makefile's `init` target covers the first apply.
  backend "gcs" {
    bucket = ""
  }
}

provider "google" {
  project = var.gcp_provider_project_id
  region  = var.gcp_provider_region
  zone    = var.gcp_provider_zone
}

# Writes the Actions variables in github_cicd.tf. Repository settings are in .github/settings.yml.
provider "github" {
  owner = var.github_repository_owner
  token = var.github_provider_token
}

# Manages the frontend Workers and their custom domains; wrangler uploads versions separately.
provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
