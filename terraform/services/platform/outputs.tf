output "uploads_bucket_name" {
  value = module.uploads_bucket.name
}

output "uploads_bucket_url" {
  value = module.uploads_bucket.url
}

output "api_service_account_email" {
  value = module.api_service_account.email
}

output "worker_service_account_email" {
  value = module.worker_service_account.email
}

output "pipeline_queue_name" {
  value = module.pipeline_queue.name
}

output "artifact_registry_url" {
  value = module.artifact_registry.repository_url
}

# Secret Manager secret IDs (synthify-<key>).
# Cloud Run secret refs need the exact secret_id.
output "secret_ids" {
  value = {
    for k, s in google_secret_manager_secret.secrets : k => s.secret_id
  }
}
