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
  description = "GitHub PAT (repo scope) for the `github` provider. Passed by hand at bootstrap only; CI sources it from Secret Manager after (see gcp_secret.tf)."
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
  description = "API token granting CI/CD access to the Cloudflare account. Passed by hand at bootstrap only; CI sources it from Secret Manager after (see gcp_secret.tf)."
  type        = string
  sensitive   = true
}

variable "google_client_id" {
  description = "Google OAuth 2.0 client ID for Google Sign-In (see https://developers.google.com/identity/gsi/web/guides/get-google-api-clientid). Defined once here and propagated via the GOOGLE_CLIENT_ID GitHub Actions variable to both the backend (infrastructure/env, as TF_VAR_google_client_id) and the frontend Docker build (VITE_GOOGLE_CLIENT_ID) for every environment. Not a secret: it identifies the app, and a token is only accepted if it was minted for it."
  type        = string
  default     = ""
}
