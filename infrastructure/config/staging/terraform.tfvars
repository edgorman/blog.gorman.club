gcp_project_id      = "blog-gorman-club-stag"
gcp_region          = "europe-west1"
environment         = "stag"
backend_cors_origin = "https://staging.blog.gorman.club"

# Only this account may use the AI writing assistant today. It is matched on the address Google
# verified in the ID token rather than on the username the profile holds: a username is freely
# chosen and, once released, claimable by anybody, so a list naming one would follow the name
# rather than the account. There is no expiry and no tier - access is configuration rather than
# something bought, and this list is the seam the real entitlement check replaces later.
assistant_allowed_emails = ["ejgorman@gmail.com"]
