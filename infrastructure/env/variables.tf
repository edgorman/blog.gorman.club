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
  description = "Placeholder image for the Cloud Run service's initial creation; CI deploys the real image afterward (drift ignored, see cloud_run.tf)."
  type        = string
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}
