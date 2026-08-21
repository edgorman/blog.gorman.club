output "environment_project_ids" {
  description = "GCP project ids for root and each environment"
  value       = { for k, v in local.all_projects : k => v.project_id }
}

output "terraform_state_buckets" {
  description = "GCS bucket names holding Terraform state for root and each environment"
  value       = { for k, v in google_storage_bucket.gcp_project_terraform_states : k => v.name }
}

output "workload_identity_provider" {
  description = "Full resource name of the WIF provider (also written to the WORKLOAD_IDENTITY_PROVIDER Actions variable)"
  value       = google_iam_workload_identity_pool_provider.github_provider.name
}

output "github_actions_service_account_email" {
  description = "Email of the GitHub Actions service account (also written to the SERVICE_ACCOUNT Actions variable)"
  value       = google_service_account.github_actions.email
}

output "frontend_domains" {
  description = "Custom domain bound to each environment's Cloudflare Pages project"
  value       = { for k, v in cloudflare_pages_domain.frontend : k => v.name }
}
