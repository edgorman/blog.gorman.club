gcp_project_id      = "blog-gorman-club-stag"
gcp_region          = "europe-west1"
environment         = "stag"
backend_cors_origin = "https://staging.blog.gorman.club"

# Only this account may use the AI writing assistant today. There is no expiry and no tier: access
# is configuration rather than something bought, and the list is the seam the real entitlement
# check replaces later.
assistant_allowed_usernames = ["edgorman"]
