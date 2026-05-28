# The uploads bucket is mounted here via gcsfuse. The worker reads source
# files as local files under this path (see GCS_FUSE_MOUNT_PATH below); there
# is no HTTP fallback, so the mount and the env var must stay in lockstep.
locals {
  gcs_fuse_mount_path = "/mnt/gcs/${var.uploads_bucket_name}"
}

module "service" {
  source = "../../modules/cloud_run_service"

  project_id            = var.project_id
  region                = var.region
  name                  = var.name
  image                 = var.image
  service_account_email = var.service_account_email
  allow_unauthenticated = false
  # ingress is INGRESS_TRAFFIC_ALL: worker is invoked from the API service,
  # which does not have a VPC connector, so internal-only ingress blocks all
  # Cloud Run -> Cloud Run traffic and returns HTML 404. Access is gated by
  # allow_unauthenticated=false + IAM roles/run.invoker bound only to the API
  # service account.
  deletion_protection = var.deletion_protection

  gcs_volume = {
    bucket     = var.uploads_bucket_name
    mount_path = local.gcs_fuse_mount_path
  }

  # PORT is a Cloud Run reserved env var: it is injected automatically from
  # the container port (cloud_run_service container_port, default 8080) and
  # must not be set explicitly. The app reads PORT with an 8080 fallback.
  env_vars = {
    SERVICE_MODE = "worker"
    # Base URL is scheme+host only (no bucket path). ValidateBaseURL rejects
    # a path, and BuildDocumentUploadURL inserts the bucket from GCS_BUCKET.
    GCS_BUCKET                   = var.uploads_bucket_name
    GCS_UPLOAD_URL_BASE          = "https://storage.googleapis.com"
    INTERNAL_GCS_UPLOAD_URL_BASE = "https://storage.googleapis.com"
    GCS_FUSE_MOUNT_PATH          = local.gcs_fuse_mount_path
    FIREBASE_PROJECT_ID          = var.firebase_project_id
    GEMINI_MODEL                 = var.gemini_model
    GCP_PROJECT                  = var.project_id
    VERTEX_LOCATION              = var.vertex_location
    API_BASE_URL                 = var.api_base_url
    NEW_RELIC_APP_NAME           = var.new_relic_app_name
  }

  sensitive_env_vars = {
    SYNTHIFY_READINESS_KEY = var.readiness_api_key
  }

  secret_env_vars = [
    {
      name   = "DATABASE_DSN"
      secret = var.secret_ids["database-dsn"]
    },
    {
      name   = "GEMINI_API_KEY"
      secret = var.secret_ids["gemini-api-key"]
    },
    {
      name   = "INTERNAL_WORKER_TOKEN"
      secret = var.secret_ids["internal-worker-token"]
    },
    {
      name   = "NEW_RELIC_LICENSE_KEY"
      secret = var.secret_ids["new-relic-license-key"]
    },
  ]
}
