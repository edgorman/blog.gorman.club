gcp_provider_project_id = "blog-gorman-club-root"
gcp_provider_region     = "europe-west1"
gcp_provider_zone       = "europe-west1-b"
gcp_project_prefix      = "blog-gorman-club"
gcp_projects            = ["stag", "prod"]
github_repository_owner = "edgorman"
github_repository_name  = "blog.gorman.club"

# google_client_id is created by hand once (Google Cloud Console > Credentials > OAuth client ID,
# in the root project) and set here; see services/frontend/README.md.
# google_client_id = "....apps.googleusercontent.com"

# the following are sensitive/unknown initially and thus have no defualt
# github_provider_token
# cloudflare_account_id
# cloudflare_api_token
# — pass it at apply time, e.g.:
#   TF_VAR_github_provider_token=<PAT> TF_VAR_cloudflare_account_id=<ID> TF_VAR_cloudflare_api_token=<TOK> make init
