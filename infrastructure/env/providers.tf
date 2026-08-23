# Environment-level root, applied once per environment (staging/prod) - distinct from infrastructure/root, which
# provisions the shared bootstrap resources (projects, state buckets, WIF) these environments run in. Its state
# bucket already exists (created by infrastructure/root), so no local-state bootstrap is needed here.
terraform {
  required_version = "1.15.8"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.45.0"
    }
  }

  backend "gcs" {
    bucket = ""
  }
}

provider "google" {
  project = var.gcp_project_id
  region  = var.gcp_region
}
