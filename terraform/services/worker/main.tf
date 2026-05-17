module "service" {
  source = "../../modules/cloud_run_service"

  project_id            = var.project_id
  region                = var.region
  name                  = var.name
  image                 = var.image
  service_account_email = var.service_account_email
  allow_unauthenticated = false
  ingress               = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  # PORT is a Cloud Run reserved env var: it is injected automatically from
  # the container port (cloud_run_service container_port, default 8080) and
  # must not be set explicitly. The app reads PORT with an 8080 fallback.
  env_vars = {
    SERVICE_MODE                 = "worker"
    # Base URL is scheme+host only (no bucket path). ValidateBaseURL rejects
    # a path, and BuildDocumentUploadURL inserts the bucket from GCS_BUCKET.
    GCS_BUCKET                   = var.uploads_bucket_name
    GCS_UPLOAD_URL_BASE          = "https://storage.googleapis.com"
    INTERNAL_GCS_UPLOAD_URL_BASE = "https://storage.googleapis.com"
    FIREBASE_PROJECT_ID          = var.firebase_project_id
    GEMINI_MODEL                 = var.gemini_model
    API_BASE_URL                 = var.api_base_url
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
  ]
}
