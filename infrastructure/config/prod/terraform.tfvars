gcp_project_id      = "blog-gorman-club-prod"
gcp_region          = "europe-west1"
environment         = "prod"
backend_cors_origin = "https://blog.gorman.club"

# Where the monitoring alerts in infrastructure/env/monitoring.tf are sent. This grants nothing -
# it is an address to notify, not an account to admit.
alert_notification_emails = ["ejgorman@gmail.com"]
