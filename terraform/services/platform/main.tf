resource "google_project_service" "required" {
  for_each           = var.required_services
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# Every platform resource must wait for the project APIs to be enabled.
# Without this, a fresh project races API activation and fails with 403/404
# (the class of error that hit cloudscheduler.jobs.create). On an existing
# project the APIs are already on, so these depends_on are a no-op.
module "uploads_bucket" {
  source = "../../modules/gcs_bucket"

  project_id   = var.project_id
  name         = var.uploads_bucket_name
  location     = var.region
  cors_origins = var.uploads_bucket_cors_origins

  depends_on = [google_project_service.required]
}

module "api_service_account" {
  source = "../../modules/service_account"

  project_id   = var.project_id
  account_id   = "synthify-api-${var.environment}"
  display_name = "Synthify API (${var.environment})"

  depends_on = [google_project_service.required]
}

module "worker_service_account" {
  source = "../../modules/service_account"

  project_id   = var.project_id
  account_id   = "synthify-worker-${var.environment}"
  display_name = "Synthify Worker (${var.environment})"

  depends_on = [google_project_service.required]
}

module "eval_service_account" {
  source = "../../modules/service_account"

  project_id   = var.project_id
  account_id   = "synthify-eval-${var.environment}"
  display_name = "Synthify Eval (${var.environment})"

  depends_on = [google_project_service.required]
}

module "eval_scheduler_service_account" {
  source = "../../modules/service_account"

  project_id   = var.project_id
  account_id   = "synthify-eval-scheduler-${var.environment}"
  display_name = "Synthify Eval Scheduler (${var.environment})"

  depends_on = [google_project_service.required]
}

module "pipeline_queue" {
  source = "../../modules/cloud_tasks_queue"

  project_id = var.project_id
  location   = var.region
  name       = "synthify-pipeline-${var.environment}"

  depends_on = [google_project_service.required]
}

module "artifact_registry" {
  source = "../../modules/artifact_registry"

  project_id    = var.project_id
  location      = var.region
  repository_id = "synthify-${var.environment}"

  depends_on = [google_project_service.required]
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
    "new-relic-license-key",
  ])

  eval_secrets = toset([
    "gemini-api-key",
  ])

  all_secrets = toset(concat(tolist(local.api_secrets), tolist(local.worker_secrets), tolist(local.eval_secrets)))
}

resource "google_secret_manager_secret" "secrets" {
  for_each  = local.all_secrets
  project   = var.project_id
  secret_id = "synthify-${each.value}"

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

resource "google_secret_manager_secret_iam_member" "eval_accessor" {
  for_each  = local.eval_secrets
  secret_id = google_secret_manager_secret.secrets[each.value].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${module.eval_service_account.email}"
}

resource "google_storage_bucket_iam_member" "eval_artifact_writer" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${module.eval_service_account.email}"
}

# The worker mounts this bucket read-only via gcsfuse and reads uploaded
# source files as local files. Without object-read access the mount exists
# but every read fails, so this binding is required, not optional.
resource "google_storage_bucket_iam_member" "worker_uploads_reader" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${module.worker_service_account.email}"
}
