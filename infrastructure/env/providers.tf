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
    # Beta-only: google_firebase_project (firestore.tf) isn't in the GA google provider yet.
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "7.45.0"
    }
    time = {
      source  = "hashicorp/time"
      version = "0.13.1"
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

provider "google-beta" {
  project = var.gcp_project_id
  region  = var.gcp_region
}
