provider "google" {
  project = var.project_id
  region  = var.region
}

module "bootstrap" {
  source = "../services/bootstrap"

  project_id                  = var.project_id
  region                      = var.region
  environment                 = var.environment
  uploads_bucket_name         = local.uploads_bucket_name
  uploads_bucket_cors_origins = local.cors_allowed_origin_list
  deployer_principal          = var.deployer_principal
  deployer_project_roles      = var.deployer_project_roles
  gcs_upload_issuer           = var.gcs_upload_issuer
}

module "worker" {
  source = "../services/worker"

  project_id            = var.project_id
  region                = var.region
  name                  = "synthify-worker-${var.environment}"
  image                 = var.worker_image
  service_account_email = module.bootstrap.worker_service_account_email
  uploads_bucket_name   = module.bootstrap.uploads_bucket_name
  secret_ids            = module.bootstrap.secret_ids
  firebase_project_id   = local.firebase_project_id
  gemini_model          = var.gemini_model
  api_base_url          = var.api_base_url
  readiness_api_key     = var.readiness_api_key
  new_relic_app_name    = local.new_relic_worker_app_name
  log_llm_payload       = var.log_llm_payload
  deletion_protection   = local.deletion_protection
}

module "eval" {
  source = "../services/eval"

  project_id                      = var.project_id
  region                          = var.region
  name                            = "synthify-eval-${var.environment}"
  image                           = var.eval_image
  service_account_email           = module.bootstrap.eval_service_account_email
  scheduler_service_account_email = module.bootstrap.eval_scheduler_service_account_email
  secret_ids                      = module.bootstrap.secret_ids
  gemini_model                    = var.gemini_model
  output_gcs_uri                  = "gs://${module.bootstrap.uploads_bucket_name}/eval/${var.environment}/runs"
  schedule                        = var.eval_schedule
  time_zone                       = var.eval_time_zone
}

module "api" {
  source = "../services/api"

  project_id            = var.project_id
  region                = var.region
  name                  = "synthify-api-${var.environment}"
  image                 = var.api_image
  service_account_email = module.bootstrap.api_service_account_email
  worker_base_url       = module.worker.uri
  # Cloud Tasks dispatch: enqueue onto pipeline_queue, push to the
  # worker's /internal/dispatch-job endpoint with an OIDC token minted
  # for the API SA (Cloud Tasks impersonates it because the SA has
  # iam.serviceAccountTokenCreator on itself).
  worker_cloudtasks_queue   = module.bootstrap.pipeline_queue_path
  worker_dispatch_url       = "${module.worker.uri}/internal/dispatch-job"
  worker_invoker_sa         = module.bootstrap.api_service_account_email
  uploads_bucket_name       = module.bootstrap.uploads_bucket_name
  secret_ids                = module.bootstrap.secret_ids
  firebase_project_id       = local.firebase_project_id
  cors_allowed_origins      = local.cors_allowed_origins
  gemini_model              = var.gemini_model
  env                       = local.env
  stripe_pro_price_id       = var.stripe_pro_price_id
  stripe_pro_price_id_jpy   = var.stripe_pro_price_id_jpy
  stripe_pro_price_id_usd   = var.stripe_pro_price_id_usd
  stripe_default_currency   = var.stripe_default_currency
  stripe_meter_input_event  = var.stripe_meter_input_event
  stripe_meter_output_event = var.stripe_meter_output_event
  billing_success_url       = local.billing_success_url
  billing_cancel_url        = local.billing_cancel_url
  billing_portal_return_url = local.billing_portal_return_url
  new_relic_app_name        = local.new_relic_app_name
  readiness_api_key         = var.readiness_api_key

  gcs_upload_issuer                 = var.gcs_upload_issuer
  gcs_signing_service_account_email = var.gcs_signing_service_account_email == "" ? module.bootstrap.api_service_account_email : var.gcs_signing_service_account_email
  gcs_signed_url_ttl_minutes        = var.gcs_signed_url_ttl_minutes
  admin_user_emails                 = var.admin_user_emails
  allowed_user_emails               = var.allowed_user_emails
  log_llm_payload                   = var.log_llm_payload
  deletion_protection               = local.deletion_protection
}

resource "google_cloud_run_v2_service_iam_member" "api_invokes_worker" {
  project  = var.project_id
  location = var.region
  name     = module.worker.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${module.bootstrap.api_service_account_email}"
}

# NOTE: the Pass-1 IAM grants (api_self_sign_blob, deployer actAs, deployer
# project roles) and the platform module now live in module.bootstrap.
#
# State for existing environments was migrated once via `terraform state mv`,
# NOT moved{} blocks. moved{} is incompatible with the Pass-1
# -target=module.bootstrap: the moved *from* addresses sit at root scope, fall
# outside the target, and Terraform hard-fails with "Moved resource instances
# excluded by targeting". A fresh environment just creates everything under
# bootstrap directly. To migrate prod, run the same state mv steps against the
# prod state once (see the bootstrap module / PR notes for the exact commands).
