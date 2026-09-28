# The two Stripe credentials the backend's billing routes need, one pair per environment: staging
# holds test-mode keys that cannot move real money, prod the live ones. Terraform creates the
# secrets and who may read them, but never a version - the values are added by hand in the console
# (or with `gcloud secrets versions add`), so a live key never passes through tfvars, plan output
# or state. Cloud Run only mounts them once stripe_price_id is set (see cloud_run.tf), which is
# the step that comes after filling them in; see "Billing" in services/backend/AGENTS.md.
locals {
  stripe_secrets = {
    stripe_secret_key     = "stripe-secret-key"
    stripe_webhook_secret = "stripe-webhook-secret"
  }
}

resource "google_secret_manager_secret" "stripe" {
  for_each = local.stripe_secrets

  project   = var.gcp_project_id
  secret_id = each.value

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_member" "backend_runtime_stripe" {
  for_each = google_secret_manager_secret.stripe

  project   = var.gcp_project_id
  secret_id = each.value.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend_runtime.email}"
}
