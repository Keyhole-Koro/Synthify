provider "google" {
  project = var.project_id
  region  = var.region
}

module "platform" {
  source = "../services/platform"

  project_id          = var.project_id
  region              = var.region
  environment         = var.environment
  uploads_bucket_name = local.uploads_bucket_name
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
  firebase_project_id   = local.firebase_project_id
  gemini_model          = var.gemini_model
  api_base_url          = var.api_base_url
  deletion_protection   = local.deletion_protection
}

module "eval" {
  source = "../services/eval"

  project_id                      = var.project_id
  region                          = var.region
  name                            = "synthify-eval-${var.environment}"
  image                           = var.eval_image
  service_account_email           = module.platform.eval_service_account_email
  scheduler_service_account_email = module.platform.eval_scheduler_service_account_email
  secret_ids                      = module.platform.secret_ids
  gemini_model                    = var.gemini_model
  schedule                        = var.eval_schedule
  time_zone                       = var.eval_time_zone
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

  gcs_upload_issuer                 = var.gcs_upload_issuer
  gcs_signing_service_account_email = var.gcs_signing_service_account_email == "" ? module.platform.api_service_account_email : var.gcs_signing_service_account_email
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
  member   = "serviceAccount:${module.platform.api_service_account_email}"
}

# Signed upload URLs use IAM SignBlob when no private key is provided
# (see packages/shared/app/bootstrap.go signBytesWithIAM). The API service
# account must be able to sign blobs as itself.
resource "google_service_account_iam_member" "api_self_sign_blob" {
  count              = var.gcs_upload_issuer == "signed" ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${module.platform.api_service_account_email}"
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${module.platform.api_service_account_email}"
}

# The principal running `terraform apply` (CI WIF SA) must be able to actAs
# the runtime SAs; GCP enforces this when attaching a SA to a Cloud Run
# service. Empty deployer_principal => skip (local apply by a broad-IAM human).
resource "google_service_account_iam_member" "deployer_acts_as_worker" {
  count              = var.deployer_principal == "" ? 0 : 1
  service_account_id = "projects/${var.project_id}/serviceAccounts/${module.platform.worker_service_account_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = var.deployer_principal
}

resource "google_service_account_iam_member" "deployer_acts_as_eval" {
  count              = var.deployer_principal == "" ? 0 : 1
  service_account_id = "projects/${var.project_id}/serviceAccounts/${module.platform.eval_service_account_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = var.deployer_principal
}

resource "google_service_account_iam_member" "deployer_acts_as_eval_scheduler" {
  count              = var.deployer_principal == "" ? 0 : 1
  service_account_id = "projects/${var.project_id}/serviceAccounts/${module.platform.eval_scheduler_service_account_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = var.deployer_principal
}

resource "google_service_account_iam_member" "deployer_acts_as_api" {
  count              = var.deployer_principal == "" ? 0 : 1
  service_account_id = "projects/${var.project_id}/serviceAccounts/${module.platform.api_service_account_email}"
  role               = "roles/iam.serviceAccountUser"
  member             = var.deployer_principal
}
