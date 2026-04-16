resource "google_project_service" "required" {
  for_each           = var.required_services
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

module "uploads_bucket" {
  source = "../../modules/gcs_bucket"

  project_id = var.project_id
  name       = var.uploads_bucket_name
  location   = var.region
}

module "api_service_account" {
  source = "../../modules/service_account"

  project_id   = var.project_id
  account_id   = "synthify-api-${var.environment}"
  display_name = "Synthify API (${var.environment})"
}

module "worker_service_account" {
  source = "../../modules/service_account"

  project_id   = var.project_id
  account_id   = "synthify-worker-${var.environment}"
  display_name = "Synthify Worker (${var.environment})"
}

module "pipeline_queue" {
  source = "../../modules/cloud_tasks_queue"

  project_id = var.project_id
  location   = var.region
  name       = "synthify-pipeline-${var.environment}"
}
