gcp_project_id      = "blog-gorman-club-prod"
gcp_region          = "europe-west1"
environment         = "prod"
backend_cors_origin = "https://blog.gorman.club"

# Where the monitoring alerts in infrastructure/env/monitoring.tf are sent. This grants nothing -
# it is an address to notify, not an account to admit. Who may use the AI writing assistant is not
# configured here at all: an account is entitled while its own subscription is live
# (subscribedUntil on its profile), so access is granted by writing that field rather than by a
# deploy.
alert_notification_emails = ["ejgorman@gmail.com"]
