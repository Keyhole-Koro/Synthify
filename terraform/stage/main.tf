provider "google" {
  project = var.project_id
  region  = var.region
}

module "platform" {
  source = "../services/platform"

  project_id          = var.project_id
  region              = var.region
  environment         = var.environment
  uploads_bucket_name = var.uploads_bucket_name
}

module "worker" {
  source = "../services/worker"

  project_id            = var.project_id
  region                = var.region
  name                  = "synthify-worker-${var.environment}"
  image                 = var.worker_image
  service_account_email = module.platform.worker_service_account_email
  uploads_bucket_name   = module.platform.uploads_bucket_name
  database_url_secret   = var.database_url_secret
  worker_token_secret   = var.worker_token_secret
  gemini_api_key_secret = var.gemini_api_key_secret
  firebase_project_id   = var.firebase_project_id
  gemini_model          = var.gemini_model
}

module "api" {
  source = "../services/api"

  project_id            = var.project_id
  region                = var.region
  name                  = "synthify-api-${var.environment}"
  image                 = var.api_image
  service_account_email = module.platform.api_service_account_email
  worker_base_url       = module.worker.uri
  uploads_bucket_name   = module.platform.uploads_bucket_name
  database_url_secret   = var.database_url_secret
  worker_token_secret   = var.worker_token_secret
  gemini_api_key_secret = var.gemini_api_key_secret
  firebase_project_id   = var.firebase_project_id
  cors_allowed_origins  = var.cors_allowed_origins
  gemini_model          = var.gemini_model
}
