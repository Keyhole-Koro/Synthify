output "api_uri" {
  value = module.api.uri
}

output "worker_uri" {
  value = module.worker.uri
}

output "uploads_bucket_name" {
  value = module.platform.uploads_bucket_name
}

output "pipeline_queue_name" {
  value = module.platform.pipeline_queue_name
}
