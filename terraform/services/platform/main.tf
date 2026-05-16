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

# --------------------------------------------------------------------
# Secret Manager
#
# DB は外部 (CockroachDB Serverless) 管理。DATABASE_DSN は CockroachDB の接続文字列を
# 手動で `gcloud secrets versions add` する運用。
# --------------------------------------------------------------------

locals {
  api_secrets = toset([
    "database-dsn",
    "gemini-api-key",
    "stripe-secret-key",
    "stripe-webhook-secret",
    "new-relic-license-key",
    "internal-worker-token",
  ])

  worker_secrets = toset([
    "database-dsn",
    "gemini-api-key",
    "internal-worker-token",
  ])

  all_secrets = toset(concat(tolist(local.api_secrets), tolist(local.worker_secrets)))
}

resource "google_secret_manager_secret" "secrets" {
  for_each  = local.all_secrets
  project   = var.project_id
  secret_id = "synthify-${each.value}-${var.environment}"

  replication {
    auto {}
  }

  depends_on = [google_project_service.required]
}

resource "google_secret_manager_secret_iam_member" "api_accessor" {
  for_each  = local.api_secrets
  secret_id = google_secret_manager_secret.secrets[each.value].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${module.api_service_account.email}"
}

resource "google_secret_manager_secret_iam_member" "worker_accessor" {
  for_each  = local.worker_secrets
  secret_id = google_secret_manager_secret.secrets[each.value].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${module.worker_service_account.email}"
}
