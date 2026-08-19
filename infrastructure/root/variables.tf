variable "gcp_provider_project_id" {
  description = "The id of the root GCP project"
  type        = string
  default     = "blog-gorman-club-root"
}

variable "gcp_provider_region" {
  description = "The region of the root GCP project"
  type        = string
  default     = "europe-west1"
}

variable "gcp_provider_zone" {
  description = "The zone of the root GCP project"
  type        = string
  default     = "europe-west1-b"
}

variable "gcp_project_prefix" {
  description = "The prefix used for the environment GCP project ids"
  type        = string
  default     = "blog-gorman-club"
}

variable "gcp_projects" {
  description = "The environment GCP projects to create, in addition to root"
  type        = list(string)
  default     = ["stag", "prod"]
}

variable "github_provider_token" {
  description = "GitHub PAT (repo scope) used by the `github` provider to write the WORKLOAD_IDENTITY_PROVIDER and SERVICE_ACCOUNT Actions variables. Passed by hand for the one-time manual bootstrap; every apply after that (including CI's) sources it from the github_provider_token secret in Secret Manager instead (see gcp_secret.tf), so it's never stored as a GitHub Actions secret."
  type        = string
  sensitive   = true
}

variable "github_repository_owner" {
  description = "The owner of the GitHub repository"
  type        = string
  default     = "edgorman"
}

variable "github_repository_name" {
  description = "The name of the GitHub repository"
  type        = string
  default     = "blog.gorman.club"
}

variable "cloudflare_account_id" {
  description = "The id of the cloudflare account."
  type        = string
}

variable "cloudflare_api_token" {
  description = "The api token granting CICD access to the cloudflare account. Passed by hand for the one-time manual bootstrap; every apply after that (including CI's) sources it from the cloudflare_api_token secret in Secret Manager instead (see gcp_secret.tf), so it's never stored as a GitHub Actions secret."
  type        = string
  sensitive   = true
}
