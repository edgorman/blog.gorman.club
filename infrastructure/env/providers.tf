# Environment-level root, applied once per environment (staging/prod) - distinct from infrastructure/root, which
# provisions the shared bootstrap resources (projects, state buckets, WIF) these environments run in. Its state
# bucket already exists (created by infrastructure/root), so no local-state bootstrap is needed here.
terraform {
  required_version = "1.15.8"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "7.45.0"
    }
    # Beta-only: google_firebase_project (firestore.tf) isn't in the GA google provider yet.
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "7.45.0"
    }
  }

  backend "gcs" {
    bucket = ""
  }
}

provider "google" {
  project = var.gcp_project_id
  region  = var.gcp_region
}

provider "google-beta" {
  project = var.gcp_project_id
  region  = var.gcp_region
}

# CI authenticates as a service account that lives in the *root* project, so GCP attributes API
# quota to root by default - but the Firebase/Firestore APIs are enabled on this environment's
# project, not root. That mismatch is what produced "Firebase Management API has not been used in
# project <root's number>" even though Terraform had just enabled it on the stag/prod project.
#
# user_project_override sends an X-Goog-User-Project header so quota is attributed to the project
# actually being modified. It requires serviceusage.services.use on that project, granted in
# infrastructure/root/github_cicd.tf. Scoped to an alias so only the resources that need it are
# affected - enabling the APIs themselves (google_project_service) must not depend on a project
# whose APIs aren't enabled yet.
provider "google" {
  alias                 = "firebase"
  project               = var.gcp_project_id
  region                = var.gcp_region
  user_project_override = true
}

provider "google-beta" {
  alias                 = "firebase"
  project               = var.gcp_project_id
  region                = var.gcp_region
  user_project_override = true
}
