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

module "artifact_registry" {
  source = "../../modules/artifact_registry"

  project_id    = var.project_id
  location      = var.region
  repository_id = "synthify-${var.environment}"
}

resource "google_secret_manager_secret" "database_url" {
  secret_id = "synthify-database-url-${var.environment}"
  project   = var.project_id

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "gemini_api_key" {
  secret_id = "synthify-gemini-api-key-${var.environment}"
  project   = var.project_id

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_iam_member" "api_db_secret_accessor" {
  secret_id = google_secret_manager_secret.database_url.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${module.api_service_account.email}"
}

resource "google_secret_manager_secret_iam_member" "api_gemini_secret_accessor" {
  secret_id = google_secret_manager_secret.gemini_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${module.api_service_account.email}"
}

resource "google_secret_manager_secret_iam_member" "worker_db_secret_accessor" {
  secret_id = google_secret_manager_secret.database_url.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${module.worker_service_account.email}"
}

resource "google_secret_manager_secret_iam_member" "worker_gemini_secret_accessor" {
  secret_id = google_secret_manager_secret.gemini_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${module.worker_service_account.email}"
}
