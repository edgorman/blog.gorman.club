output "backend_url" {
  description = "Public URL of the deployed Cloud Run backend service"
  value       = google_cloud_run_v2_service.backend.uri
}

output "firestore_database" {
  description = "Full resource name of the Firestore database"
  value       = google_firestore_database.database.name
}

output "uptime_check_id" {
  description = "Id of the uptime check polling the backend's /health endpoint"
  value       = google_monitoring_uptime_check_config.backend_health.uptime_check_id
}

output "alert_policy_names" {
  description = "Full resource names of the monitoring alert policies watching the backend"
  value = {
    uptime     = google_monitoring_alert_policy.backend_uptime.name
    error_rate = google_monitoring_alert_policy.backend_error_rate.name
    latency    = google_monitoring_alert_policy.backend_latency.name
  }
}
