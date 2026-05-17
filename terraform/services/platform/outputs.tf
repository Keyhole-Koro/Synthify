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

output "eval_service_account_email" {
  value = module.eval_service_account.email
}

output "eval_scheduler_service_account_email" {
  value = module.eval_scheduler_service_account.email
}

output "pipeline_queue_name" {
  value = module.pipeline_queue.name
}

output "artifact_registry_url" {
  value = module.artifact_registry.repository_url
}

# Secret Manager secret IDs (synthify-<key>).
# Cloud Run secret refs need the exact secret_id.
#
# Falls back to the deterministic "synthify-<key>" name for any key not yet
# in state. This keeps consumers (api/worker) evaluable during bootstrap /
# `terraform import`, where the secrets resource may be partially populated.
# Once a secret is in state its real secret_id (identical by construction)
# takes precedence via the merge.
output "secret_ids" {
  value = merge(
    { for k in local.all_secrets : k => "synthify-${k}" },
    { for k, s in google_secret_manager_secret.secrets : k => s.secret_id },
  )
}
