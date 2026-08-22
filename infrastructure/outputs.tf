output "backend_url" {
  description = "Public URL of the deployed Cloud Run backend service"
  value       = google_cloud_run_v2_service.backend.uri
}

output "artifact_registry_repository" {
  description = "Full resource name of the backend Artifact Registry repository"
  value       = google_artifact_registry_repository.backend.name
}
