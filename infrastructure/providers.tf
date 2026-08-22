# Environment-level Terraform root: the "Centralized Terraform manifests
# using environment-specific variable configurations" described in
# CLAUDE.md's Repository Structure section. Applied once per environment
# (infrastructure/config/staging, infrastructure/config/prod) - distinct
# from infrastructure/root, which provisions the shared/bootstrap resources
# (the projects themselves, state buckets, WIF) those environments run in.
#
# Unlike infrastructure/root, this root's state bucket already exists by the
# time it's first applied - infrastructure/root's gcp_project.tf creates a
# `<project_id>-terraform-state` bucket for every environment project, root
# included - so there's no local-state bootstrap dance here.
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
