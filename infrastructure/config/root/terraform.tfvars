gcp_provider_project_id = "blog-gorman-club-root"
gcp_provider_region     = "europe-west1"
gcp_provider_zone       = "europe-west1-b"
gcp_project_prefix      = "blog-gorman-club"
gcp_projects            = ["staging", "prod"]
github_repository_owner = "edgorman"
github_repository_name  = "blog.gorman.club"

# github_provider_token is deliberately not set here (sensitive, no default)
# — pass it at apply time, e.g.:
#   TF_VAR_github_provider_token=<PAT> make init
