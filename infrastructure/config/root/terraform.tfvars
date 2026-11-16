gcp_provider_project_id = "blog-gorman-club-root"
gcp_provider_region     = "europe-west1"
gcp_provider_zone       = "europe-west1-b"
gcp_project_prefix      = "blog-gorman-club"
gcp_projects            = ["stag", "prod"]
github_repository_owner = "edgorman"
github_repository_name  = "blog.gorman.club"

# Created by hand once (Google Cloud Console > Credentials > OAuth client ID, in the root
# project) and set here; see services/frontend/README.md. Not a secret - the ID-token flow this
# app uses has no client secret, and the ID itself ships in the frontend bundle and every
# browser's network tab regardless.
google_client_id = "671777226474-sqa0ceudduou1rg0cloe3q4f46nmelvv.apps.googleusercontent.com"

# From Google Search Console's DNS verification for the gorman.club Domain property (#221). Not a
# secret - it is published in DNS as a TXT record anyway.
google_site_verification = "Zp_UP1vLDTpUNWsHeNP3xGScmS8c4q5wJw0SPat6j38"

# Cloudflare dashboard > the gorman.club zone's Overview page: the account id and the zone id sit
# in the right-hand column. Neither is a secret. Both are read at plan time, so they live here
# rather than in Secret Manager (decision 38). TODO(Ed): fill both in before the first apply; the
# validation on each variable fails the plan until they are 32-character ids.
cloudflare_account_id = ""
cloudflare_zone_id    = ""

cloudflare_zone = "gorman.club"
frontend_hostnames = {
  stag = "staging.blog.gorman.club"
  prod = "blog.gorman.club"
}

# The following are sensitive and thus have no default
# github_provider_token
# cloudflare_api_token
# - pass them at bootstrap only, e.g.:
#   TF_VAR_github_provider_token=<PAT> TF_VAR_cloudflare_api_token=<TOK> make init
# Afterwards they live in Secret Manager and a rotation is a new secret version.
