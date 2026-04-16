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
