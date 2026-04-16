module "service" {
  source = "../../modules/cloud_run_service"

  project_id            = var.project_id
  region                = var.region
  name                  = var.name
  image                 = var.image
  service_account_email = var.service_account_email
  allow_unauthenticated = false
  ingress               = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  env_vars = {
    SERVICE_MODE                 = "worker"
    PORT                         = "8080"
    GCS_BUCKET                   = var.uploads_bucket_name
    GCS_UPLOAD_URL_BASE          = "https://storage.googleapis.com/${var.uploads_bucket_name}"
    INTERNAL_GCS_UPLOAD_URL_BASE = "https://storage.googleapis.com/${var.uploads_bucket_name}"
    FIREBASE_PROJECT_ID          = var.firebase_project_id
  }

  secret_env_vars = [
    {
      name   = "DATABASE_URL"
      secret = var.database_url_secret
    },
    {
      name   = "INTERNAL_WORKER_TOKEN"
      secret = var.worker_token_secret
    }
  ]
}
