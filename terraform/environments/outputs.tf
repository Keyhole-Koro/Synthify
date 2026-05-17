output "api_uri" {
  value = module.api.uri
}

output "worker_uri" {
  value = module.worker.uri
}

output "eval_job_name" {
  value = module.eval.name
}

output "eval_schedule_name" {
  value = module.eval.schedule_name
}

output "uploads_bucket_name" {
  value = module.bootstrap.uploads_bucket_name
}

output "pipeline_queue_name" {
  value = module.bootstrap.pipeline_queue_name
}

output "project_id" {
  value = var.project_id
}

output "environment" {
  value = var.environment
}
