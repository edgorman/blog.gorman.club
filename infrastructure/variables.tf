variable "gcp_project_id" {
  description = "The GCP project this environment's resources are created in (e.g. blog-gorman-club-stag)"
  type        = string
}

variable "gcp_region" {
  description = "The region Cloud Run and Artifact Registry resources are created in"
  type        = string
  default     = "europe-west1"
}

variable "environment" {
  description = "Short environment name used in resource naming (e.g. backend-stag) and passed to the backend as its ENVIRONMENT env var"
  type        = string
}

variable "backend_cors_origin" {
  description = "Origin allowed to call the backend from a browser - this environment's frontend URL"
  type        = string
}

variable "backend_initial_image" {
  description = "Placeholder image used only the first time the Cloud Run service is created. CI deploys the real image on every push after that, and Terraform ignores drift on it (see cloud_run.tf) so it never reverts a deploy."
  type        = string
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}
