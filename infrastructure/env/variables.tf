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

variable "embedding_model" {
  description = "Text-embedding model the worker embeds posts with, served by the Agent Platform's :predict method in embedding_location. Changing it re-embeds every post on the worker's next start, since vectors from different models are not comparable. Empty disables embeddings."
  type        = string
  default     = "gemini-embedding-001"
}

variable "embedding_location" {
  description = "Location embedding_model is called in: a region such as europe-west1, or \"global\" for the multi-region endpoint."
  type        = string
  default     = "europe-west1"
}

variable "embedding_dimension" {
  description = "Length of every post embedding, and of the vector index on the embeddings collection that must match it. At most 2048, Firestore's limit for an indexed vector. Changing it needs every embedding rewritten, which changing embedding_model alongside it does."
  type        = number
  default     = 768
}

variable "alert_notification_emails" {
  description = "Addresses the monitoring alerts in monitoring.tf are sent to. Empty leaves the policies in place but silent - they still show in the console, nobody is told. These are recipients rather than an entitlement: an address here is where a message goes, not an account admitted to anything."
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

variable "alert_client_error_count_threshold" {
  description = "How many 4xx responses in a five minute window the backend may serve before alerting. Deliberately high: 404s from stale or mistyped links are ordinary traffic, so this is meant to catch floods of 401s or 429s, not browsing."
  type        = number
  default     = 100
}

variable "alert_rate_limited_count_threshold" {
  description = "How many 429 responses in a five minute window the backend may serve before alerting. A 429 is only ever the backend's own rate limiter refusing a caller, so any sustained number of them means someone is calling the API far faster than using the site would."
  type        = number
  default     = 20
}

variable "alert_assistant_turn_threshold" {
  description = "How many writing assistant turns in an hour, across every caller, before alerting. Sized to one author working steadily with room to spare (the per-account rate limit refills a turn every 30 seconds, so a single caller could reach 120): more than this is several people at once, a scripted client, or a loop."
  type        = number
  default     = 60
}

variable "alert_assistant_error_count_threshold" {
  description = "How many failed writing assistant turns in a five minute window before alerting. 0 alerts on the first one: a failed turn is a model call that was paid for or refused, and either is worth knowing about on a site this quiet."
  type        = number
  default     = 0
}

variable "backend_registry_keep_count" {
  description = "How many of the most recent Artifact Registry versions the backend repository keeps (each environment's own registry, since every merge writes to both - see Staging Deployments in .github/AGENTS.md); everything older is deleted by the repository's cleanup_policies. See the Artifact Stores section of infrastructure/AGENTS.md for how this number was chosen."
  type        = number
  default     = 30
}

variable "frontend_retention_days" {
  description = "How many days a commit-SHA folder survives in the frontend bucket (each environment's own bucket, since every merge writes to both - see Staging Deployments in .github/AGENTS.md) before the bucket's lifecycle rule deletes it. See the Artifact Stores section of infrastructure/AGENTS.md for how this number was chosen."
  type        = number
  default     = 90
}

variable "worker_max_instances" {
  description = "Upper bound on worker instances. Events arrive one per post write or new comment, so a handful is plenty, and a low bound caps what a retry storm can cost."
  type        = number
  default     = 3
}
