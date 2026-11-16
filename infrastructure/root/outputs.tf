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

output "github_service_accounts" {
  description = "Emails of the GitHub Actions identities: plan (read-only), deploy (main, push-commit.yaml), promote (main, prod) and renovate. Also written to the *_SERVICE_ACCOUNT Actions variables."
  value = {
    plan     = google_service_account.github_plan.email
    deploy   = google_service_account.github_deploy.email
    promote  = google_service_account.github_promote.email
    renovate = google_service_account.github_renovate.email
  }
}

output "github_actions_service_account_email" {
  description = "Legacy single GitHub Actions service account, kept for one release (github_actions_legacy.tf)"
  value       = google_service_account.github_actions.email
}

output "frontend_workers_dev_urls" {
  description = "workers.dev address of each environment's Cloudflare Worker"
  value       = { for k, v in cloudflare_worker.frontend : k => v.subdomain.url }
}

output "frontend_domains" {
  description = "Custom domain bound to each environment's Cloudflare Worker"
  value       = { for k, v in cloudflare_workers_custom_domain.frontend : k => v.hostname }
}
