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

variable "google_client_id" {
  description = "Google OAuth 2.0 client ID the backend verifies ID tokens against. Defined once in infrastructure/root and passed in by CI as TF_VAR_google_client_id."
  type        = string
  default     = ""
}

variable "assistant_model" {
  description = "Model id the writing assistant calls, e.g. gemini-3.7-flash. It must be a model the Gemini Enterprise Agent Platform serves in assistant_location - model ids come and go faster than this service is redeployed, so this is configuration rather than a constant in the backend. Empty disables the feature."
  type        = string
  default     = "gemini-3.7-flash"
}

variable "assistant_location" {
  description = "Location the model is called in: a region such as europe-west1, or \"global\" for the multi-region endpoint. Model availability is regional, so this is what moves a deployment onto an endpoint that actually serves assistant_model."
  type        = string
  default     = "global"
}

variable "assistant_allowed_emails" {
  description = "Verified Google account addresses permitted to use the AI writing assistant. Everybody else is refused by the backend, whatever they own. Empty disables the feature for everyone. This is the placeholder for real entitlements: when access becomes something bought rather than configured, it is replaced by a per-user record carrying a tier and an expiry (see internal/entity/assistant.go)."
  type        = list(string)
  default     = []
}

variable "alert_notification_emails" {
  description = "Addresses the monitoring alerts in monitoring.tf are sent to. Empty leaves the policies in place but silent - they still show in the console, nobody is told. These are recipients rather than an entitlement, so unlike assistant_allowed_emails there is nothing to verify: an address here is simply where a message goes."
  type        = list(string)
  default     = []
}

variable "alert_error_count_threshold" {
  description = "How many 5xx responses in a five minute window the backend may serve before alerting. Counted rather than expressed as a rate because traffic here is low enough that any rate reads as noise."
  type        = number
  default     = 5
}

variable "alert_latency_threshold_ms" {
  description = "The 95th percentile request latency, in milliseconds, the backend may exceed for ten minutes before alerting. Set above a cold start on purpose: the service scales to zero, so seconds-long first requests are normal and alerting under this would page for them."
  type        = number
  default     = 5000
}
