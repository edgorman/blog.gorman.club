gcp_project_id      = "blog-gorman-club-prod"
gcp_region          = "europe-west1"
environment         = "prod"
backend_cors_origin = "https://blog.gorman.club"

# Only this account may use the AI writing assistant today. It is matched on the address Google
# verified in the ID token rather than on the username the profile holds: a username is freely
# chosen and, once released, claimable by anybody, so a list naming one would follow the name
# rather than the account. There is no expiry and no tier - access is configuration rather than
# something bought, and this list is the seam the real entitlement check replaces later.
assistant_allowed_emails = ["ejgorman@gmail.com"]

# Where the monitoring alerts in infrastructure/env/monitoring.tf are sent. Unlike the list above
# this one grants nothing - it is an address to notify, not an account to admit.
alert_notification_emails = ["ejgorman@gmail.com"]

# The budget alert is off until the billing account grants CI billing.costsManager; see the
# budget_amount description in infrastructure/env/variables.tf for the one command that does it.
# Set the monthly figure here afterwards, in the billing account's own currency:
# budget_amount = 10
