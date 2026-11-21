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

# The runtime service accounts are created in terraform/manual/iam, applied
# by hand: the deploy account cannot create service accounts. Their emails
# follow from the names.
locals {
  service_account_emails = {
    for key in ["api", "worker", "eval", "eval_scheduler", "monitor"] :
    key => "synthify-${replace(key, "_", "-")}-${var.environment}@${var.project_id}.iam.gserviceaccount.com"
  }
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
    "gemini-api-key",
  ])

  eval_secrets = toset([
    "database-dsn",
    "gemini-api-key",
  ])

  # monitor は read-only の monitor ロールを指す DSN のみ (api/worker とは別の
  # 資格情報)。MONITOR_DATABASE_URL としてマウントする。
  monitor_secrets = toset([
    "monitor-database-dsn",
  ])

  all_secrets = toset(concat(
    tolist(local.api_secrets),
    tolist(local.worker_secrets),
    tolist(local.eval_secrets),
    tolist(local.monitor_secrets),
  ))
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
  member    = "serviceAccount:${local.service_account_emails["api"]}"
}

resource "google_secret_manager_secret_iam_member" "worker_accessor" {
  for_each  = local.worker_secrets
  secret_id = google_secret_manager_secret.secrets[each.value].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.service_account_emails["worker"]}"
}

resource "google_secret_manager_secret_iam_member" "eval_accessor" {
  for_each  = local.eval_secrets
  secret_id = google_secret_manager_secret.secrets[each.value].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.service_account_emails["eval"]}"
}

resource "google_secret_manager_secret_iam_member" "monitor_accessor" {
  for_each  = local.monitor_secrets
  secret_id = google_secret_manager_secret.secrets[each.value].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.service_account_emails["monitor"]}"
}

resource "google_storage_bucket_iam_member" "eval_artifact_writer" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${local.service_account_emails["eval"]}"
}

resource "google_storage_bucket_iam_member" "api_uploads_writer" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${local.service_account_emails["api"]}"
}

resource "google_storage_bucket_iam_member" "api_uploads_reader" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${local.service_account_emails["api"]}"
}

# The worker mounts this bucket read-write via gcsfuse: it reads uploaded
# source files as local files and writes job checkpoints under .checkpoints/.
# objectViewer alone makes the mount read-only and every checkpoint write fails
# ("read-only file system"), so objectAdmin is required — checkpoints are
# created, overwritten, and deleted, which objectCreator cannot do.
resource "google_storage_bucket_iam_member" "worker_uploads_reader" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${local.service_account_emails["worker"]}"
}

resource "google_storage_bucket_iam_member" "worker_uploads_writer" {
  bucket = module.uploads_bucket.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${local.service_account_emails["worker"]}"
}

# Moved to terraform/manual/iam: the service accounts, their project roles
# (Firestore, Vertex AI, Cloud Tasks) and the API's grants on itself. They
# need projectIamAdmin and serviceAccountAdmin, which the deploy account no
# longer has. These blocks make CD drop them from its state without
# destroying them, so the accounts and grants stay while the manual stack
# takes them over. They can go once every environment has deployed this.
removed {
  from = module.api_service_account
  lifecycle {
    destroy = false
  }
}

removed {
  from = module.worker_service_account
  lifecycle {
    destroy = false
  }
}

removed {
  from = module.eval_service_account
  lifecycle {
    destroy = false
  }
}

removed {
  from = module.eval_scheduler_service_account
  lifecycle {
    destroy = false
  }
}

removed {
  from = module.monitor_service_account
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_project_iam_member.api_firestore_user
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_project_iam_member.worker_firestore_user
  lifecycle {
    destroy = false
  }
}

# Added by the invitation check (#55). A removed block for a resource that is
# not in the state does nothing, so this is safe before #55 is deployed.
removed {
  from = google_project_iam_member.api_firebaseauth_viewer
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_project_iam_member.worker_aiplatform_user
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_project_iam_member.eval_aiplatform_user
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_project_iam_member.api_cloudtasks_enqueuer
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_service_account_iam_member.api_self_user
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_service_account_iam_member.api_self_token_creator
  lifecycle {
    destroy = false
  }
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
