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
  description = "The id of the Cloudflare account"
  type        = string
}

variable "cloudflare_api_token" {
  description = "API token granting CI/CD access to the Cloudflare account. Passed by hand at bootstrap only; CI sources it from Secret Manager after (see gcp_secret.tf)."
  type        = string
  sensitive   = true
}

variable "google_client_id" {
  description = "Google OAuth 2.0 client ID for Google Sign-In, propagated to both services via the GOOGLE_CLIENT_ID Actions variable. Not a secret - it identifies the app."
  type        = string
  default     = ""
}

variable "google_site_verification" {
  description = "Token from Google Search Console's DNS verification for the gorman.club Domain property (the part after `google-site-verification=`). Not a secret."
  type        = string
}

variable "backend_registry_keep_count" {
  description = "How many of the most recent versions of each image every environment's backend Artifact Registry repository keeps; everything older is deleted by the repository's cleanup_policies. See the Artifact Stores section of infrastructure/AGENTS.md for how this number was chosen."
  type        = number
  default     = 30
}

variable "frontend_retention_days" {
  description = "How many days a commit-SHA folder survives in every environment's frontend bucket before the bucket's lifecycle rule deletes it. See the Artifact Stores section of infrastructure/AGENTS.md for how this number was chosen."
  type        = number
  default     = 90
}
