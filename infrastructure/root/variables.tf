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
  description = "The id of the Cloudflare account. Not a secret (it is in the dashboard's URLs), so it is set in terraform.tfvars and written to the CLOUDFLARE_ACCOUNT_ID Actions variable. A plan is read with placeholder tokens, so it must not come from a secret."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-f]{32}$", var.cloudflare_account_id))
    error_message = "Set cloudflare_account_id in infrastructure/config/root/terraform.tfvars to the 32-character account id from the Cloudflare dashboard."
  }
}

variable "cloudflare_zone" {
  description = "The Cloudflare zone (domain) the frontend hostnames live in"
  type        = string
  default     = "gorman.club"
}

variable "cloudflare_zone_id" {
  description = "The id of cloudflare_zone, from the zone's Overview page in the Cloudflare dashboard. Not a secret. A variable rather than a data source so a PR plan never calls Cloudflare."
  type        = string

  validation {
    condition     = can(regex("^[0-9a-f]{32}$", var.cloudflare_zone_id))
    error_message = "Set cloudflare_zone_id in infrastructure/config/root/terraform.tfvars to the 32-character zone id from the Cloudflare dashboard."
  }
}

variable "frontend_hostnames" {
  description = "Full hostname of each environment's frontend, inside cloudflare_zone. An environment left out is served only at its workers.dev address."
  type        = map(string)
  default     = {}
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
