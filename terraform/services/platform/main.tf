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
    "stripe-secret-key",
    "stripe-webhook-secret",
    "new-relic-license-key",
    "internal-worker-token",
  ])

  worker_secrets = toset([
    "database-dsn",
    "internal-worker-token",
    "new-relic-license-key",
  ])

  eval_secrets = toset([])

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

resource "google_storage_bucket_iam_member" "api_uploads_writer" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${module.api_service_account.email}"
}

resource "google_storage_bucket_iam_member" "api_uploads_reader" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${module.api_service_account.email}"
}

# The worker mounts this bucket read-only via gcsfuse and reads uploaded
# source files as local files. Without object-read access the mount exists
# but every read fails, so this binding is required, not optional.
resource "google_storage_bucket_iam_member" "worker_uploads_reader" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${module.worker_service_account.email}"
}

# Firestore job-status notifier (apps/api, apps/worker) writes to
# workspaces/{ws}/jobs/{job} via the Admin SDK. Native-mode Firestore
# access is controlled by roles/datastore.user; without it the writes
# fail with PermissionDenied even though the database exists.
resource "google_project_iam_member" "api_firestore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${module.api_service_account.email}"
}

resource "google_project_iam_member" "worker_firestore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${module.worker_service_account.email}"
}

# Vertex AI access for Gemini inference.
# The worker SA calls aiplatform.googleapis.com via the genai client; eval
# uses the same path when run in CI / Cloud Run. Without this binding the
# Vertex backend returns PERMISSION_DENIED at the first request.
resource "google_project_iam_member" "worker_aiplatform_user" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${module.worker_service_account.email}"
}

resource "google_project_iam_member" "eval_aiplatform_user" {
  project = var.project_id
  role    = "roles/aiplatform.user"
  member  = "serviceAccount:${module.eval_service_account.email}"
}

# Cloud Tasks dispatch (apps/api -> worker). The API enqueues onto
# pipeline_queue with an OIDC token minted for its own service account;
# Cloud Tasks then pushes the task to worker as that SA. roles/run.invoker
# on the worker is already granted at the environment level
# (google_cloud_run_v2_service_iam_member.api_invokes_worker), so the SA
# only needs:
#   1. cloudtasks.enqueuer to call CreateTask, and
#   2. iam.serviceAccountTokenCreator on itself so Cloud Tasks can mint an
#      OIDC token with the API SA as the audience-target identity.
resource "google_project_iam_member" "api_cloudtasks_enqueuer" {
  project = var.project_id
  role    = "roles/cloudtasks.enqueuer"
  member  = "serviceAccount:${module.api_service_account.email}"
}

resource "google_service_account_iam_member" "api_self_token_creator" {
  service_account_id = "projects/${var.project_id}/serviceAccounts/${module.api_service_account.email}"
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${module.api_service_account.email}"
}

# TTL policy: Firestore deletes workspaces/{ws}/jobs/{job} documents
# automatically once their expiresAt timestamp passes. The notifier writes
# expiresAt = now + 7d on Completed and Failed, so finished jobs are kept
# long enough for users to inspect the outcome and then garbage-collected
# without an extra Cloud Scheduler job. Running/queued jobs never set
# expiresAt and stay until they reach a terminal state.
#
# google_firestore_field is also the way you address jobId/etc fields, so we
# pin collection_group="jobs" and field="expiresAt" exactly. The
# index_config block is required by the provider even when only TTL is
# configured.
resource "google_firestore_field" "jobs_expires_at_ttl" {
  project    = var.project_id
  database   = "(default)"
  collection = "jobs"
  field      = "expiresAt"

  ttl_config {}

  # Keep the default single-field index settings; declaring ttl_config alone
  # is enough to enable the TTL policy.
  index_config {}
}
