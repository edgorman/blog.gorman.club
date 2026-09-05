# Subscription payments. Buying the assistant entitlement is the one thing this deployment does
# that needs a credential Google cannot mint for it: Stripe has no equivalent of the Workload
# Identity Federation that keeps CI and the model calls key-free, so there is an actual secret to
# hold, and this file is about holding it as narrowly as possible.
#
# Three things follow from that:
#
#   - The secrets live in Secret Manager and are mounted by Cloud Run at start-up, rather than
#     being passed as plain environment variables. That is the same reasoning that put the GitHub
#     PAT and the Cloudflare token in Secret Manager rather than in GitHub Actions secrets (see
#     infrastructure/root/gcp_secret.tf).
#   - Unlike those, the *values* are not managed here. Terraform creates the containers and the
#     access; a human adds the versions with `gcloud secrets versions add`, and the live API key
#     never passes through a tfvars file, a plan output, or Terraform state. A placeholder version
#     is created so that Cloud Run has something to mount before anybody has filled it in.
#   - They are per-environment, because they are per-Stripe-mode: staging holds test-mode keys
#     that cannot move real money, production holds live ones. The environment isolation that
#     already keeps stag off prod's Firestore keeps a test key off real customers.
#
# There is no google_project_service for Secret Manager here: it is part of the baseline root
# enables uniformly across every project (see infrastructure/root/gcp_project.tf), which is exactly
# the assumption the per-environment roots are allowed to make.

# The Stripe API key (sk_test_... in staging, sk_live_... in production).
resource "google_secret_manager_secret" "stripe_secret_key" {
  project   = var.gcp_project_id
  secret_id = "stripe-secret-key"

  replication {
    auto {}
  }
}

# The signing secret of this environment's webhook endpoint (whsec_...). It only ever verifies a
# delivery and can move no money, but it is what stands between a public URL and free
# subscriptions, so it is held exactly as carefully as the key above.
resource "google_secret_manager_secret" "stripe_webhook_secret" {
  project   = var.gcp_project_id
  secret_id = "stripe-webhook-secret"

  replication {
    auto {}
  }
}

# A version has to exist for Cloud Run to mount the secret at all, and the real one is added out of
# band - so these are placeholders, created once and then ignored. The backend treats a placeholder
# exactly as it treats an unset variable in practice: Stripe rejects it, the checkout route answers
# that it could not start, and every other route is unaffected (see services/backend/cmd/backend).
#
# ignore_changes is what keeps the real versions added afterward from being reported as drift, and
# keeps a later apply from putting the placeholder back on top of them.
resource "google_secret_manager_secret_version" "stripe_secret_key_placeholder" {
  secret      = google_secret_manager_secret.stripe_secret_key.id
  secret_data = "placeholder-replace-with-gcloud-secrets-versions-add"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

resource "google_secret_manager_secret_version" "stripe_webhook_secret_placeholder" {
  secret      = google_secret_manager_secret.stripe_webhook_secret.id
  secret_data = "placeholder-replace-with-gcloud-secrets-versions-add"

  lifecycle {
    ignore_changes = [secret_data]
  }
}

# The backend reads both at start-up, and nothing else does: not CI, which never takes a payment,
# and not the GitHub Actions service account, which deploys the image that reads them. The role is
# the accessor one on the individual secret rather than a project-wide grant, so the runtime
# identity can read these two and no secret added later.
resource "google_secret_manager_secret_iam_member" "backend_runtime_stripe_secret_key_accessor" {
  project   = var.gcp_project_id
  secret_id = google_secret_manager_secret.stripe_secret_key.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend_runtime.email}"
}

resource "google_secret_manager_secret_iam_member" "backend_runtime_stripe_webhook_secret_accessor" {
  project   = var.gcp_project_id
  secret_id = google_secret_manager_secret.stripe_webhook_secret.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend_runtime.email}"
}
