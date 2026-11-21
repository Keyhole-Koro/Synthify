# Re-export platform outputs so root consumers (api/worker/eval) reference
# module.bootstrap instead of reaching into the nested platform module.

output "uploads_bucket_name" {
  value = module.platform.uploads_bucket_name
}

output "uploads_bucket_url" {
  value = module.platform.uploads_bucket_url
}

output "api_service_account_email" {
  value = module.platform.api_service_account_email
}

output "worker_service_account_email" {
  value = module.platform.worker_service_account_email
}

output "eval_service_account_email" {
  value = module.platform.eval_service_account_email
}

output "eval_scheduler_service_account_email" {
  value = module.platform.eval_scheduler_service_account_email
}

output "maintenance_scheduler_service_account_email" {
  value = module.platform.maintenance_scheduler_service_account_email
}

output "monitor_service_account_email" {
  value = module.platform.monitor_service_account_email
}

output "pipeline_queue_name" {
  value = module.platform.pipeline_queue_name
}

output "pipeline_queue_path" {
  value = module.platform.pipeline_queue_path
}

output "artifact_registry_url" {
  value = module.platform.artifact_registry_url
}

output "secret_ids" {
  value = module.platform.secret_ids
}
