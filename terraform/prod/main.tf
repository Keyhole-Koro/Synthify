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
  secret_ids            = module.platform.secret_ids
  firebase_project_id   = var.firebase_project_id
  gemini_model          = var.gemini_model
  api_base_url          = var.api_base_url
}

module "api" {
  source = "../services/api"

  project_id                = var.project_id
  region                    = var.region
  name                      = "synthify-api-${var.environment}"
  image                     = var.api_image
  service_account_email     = module.platform.api_service_account_email
  worker_base_url           = module.worker.uri
  uploads_bucket_name       = module.platform.uploads_bucket_name
  secret_ids                = module.platform.secret_ids
  firebase_project_id       = var.firebase_project_id
  cors_allowed_origins      = var.cors_allowed_origins
  gemini_model              = var.gemini_model
  env                       = var.env
  stripe_pro_price_id       = var.stripe_pro_price_id
  stripe_pro_price_id_jpy   = var.stripe_pro_price_id_jpy
  stripe_pro_price_id_usd   = var.stripe_pro_price_id_usd
  stripe_default_currency   = var.stripe_default_currency
  stripe_meter_input_event  = var.stripe_meter_input_event
  stripe_meter_output_event = var.stripe_meter_output_event
  billing_success_url       = var.billing_success_url
  billing_cancel_url        = var.billing_cancel_url
  billing_portal_return_url = var.billing_portal_return_url
  new_relic_app_name        = var.new_relic_app_name
}

resource "google_cloud_run_v2_service_iam_member" "api_invokes_worker" {
  project  = var.project_id
  location = var.region
  name     = module.worker.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${module.platform.api_service_account_email}"
}
