variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "environment" {
  description = "Resource-name suffix and Secret Manager suffix (stage | prod)"
  type        = string
}

variable "api_image" {
  description = "API container image. Passed by CD via -var after build&push; not stored in tfvars."
  type        = string
  default     = ""
}

variable "worker_image" {
  description = "Worker container image. Passed by CD via -var after build&push; not stored in tfvars."
  type        = string
  default     = ""
}

variable "eval_image" {
  description = "Eval Cloud Run Job container image. Passed by CD via -var after build&push; not stored in tfvars."
  type        = string
  default     = ""
}

# Public web origin (no trailing slash), e.g. https://stage.synthify.keyhole.work.
# CORS + billing redirect URLs are derived from this in locals.tf.
variable "web_base_url" {
  description = "Public web app origin. Drives cors_allowed_origins and billing_* URLs unless those are set explicitly."
  type        = string
}

# The following are derived in locals.tf when left empty. Set them only to
# override the derived value.
variable "uploads_bucket_name" {
  description = "Empty => <project_id>-synthify-uploads-<environment>."
  type        = string
  default     = ""
}

variable "firebase_project_id" {
  description = "Empty => same as project_id."
  type        = string
  default     = ""
}

variable "cors_allowed_origins" {
  description = "Empty => web_base_url."
  type        = string
  default     = ""
}

variable "gemini_model" {
  type    = string
  default = "gemini-3-flash-preview"
}

variable "eval_schedule" {
  description = "Cloud Scheduler cron expression for LLM eval Cloud Run Job."
  type        = string
  default     = "0 4 * * *"
}

variable "eval_time_zone" {
  type    = string
  default = "Asia/Tokyo"
}

variable "stripe_pro_price_id" {
  type    = string
  default = ""
}

variable "stripe_pro_price_id_jpy" {
  type    = string
  default = ""
}

variable "stripe_pro_price_id_usd" {
  type    = string
  default = ""
}

variable "stripe_default_currency" {
  type    = string
  default = "jpy"
}

variable "stripe_meter_input_event" {
  type    = string
  default = ""
}

variable "stripe_meter_output_event" {
  type    = string
  default = ""
}

# Empty => derived from web_base_url in locals.tf.
variable "billing_success_url" {
  description = "Empty => <web_base_url>/workspaces?billing=success."
  type        = string
  default     = ""
}

variable "billing_cancel_url" {
  description = "Empty => <web_base_url>/workspaces?billing=cancel."
  type        = string
  default     = ""
}

variable "billing_portal_return_url" {
  description = "Empty => <web_base_url>/workspaces."
  type        = string
  default     = ""
}

variable "new_relic_app_name" {
  description = "Empty => prod: synthify-api, else synthify-api-<environment>."
  type        = string
  default     = ""
}

variable "api_base_url" {
  description = "Public URL for the API service. Apply once with default empty, then set to the api Cloud Run URL and re-apply so the worker can call back."
  type        = string
  default     = ""
}

# --------------------------------------------------------------------
# Signed upload URLs
# --------------------------------------------------------------------

variable "gcs_upload_issuer" {
  description = "'signed' uses real GCS signed URLs (IAM SignBlob). Anything else uses the fake/dev issuer."
  type        = string
  default     = "signed"
}

variable "gcs_signing_service_account_email" {
  description = "SA email used to sign upload URLs. Empty => use the API service account (recommended)."
  type        = string
  default     = ""
}

variable "gcs_signed_url_ttl_minutes" {
  type    = string
  default = "15"
}

variable "admin_user_emails" {
  description = "Comma-separated list of admin user emails (SYNTHIFY_ADMIN_USER_EMAILS)."
  type        = string
  default     = ""
}

# Access allowlist. Non-empty => only these emails may use the API.
# Typically supplied per-env by CD from a GitHub Environment variable
# (empty for prod => unrestricted; set for stage to lock it down).
variable "allowed_user_emails" {
  description = "Comma-separated allowlist (SYNTHIFY_ALLOWED_USER_EMAILS). Empty => no restriction."
  type        = string
  default     = ""
}

variable "log_llm_payload" {
  description = "Set to \"true\" to log raw LLM payloads (debug only)."
  type        = string
  default     = "false"
}

# Principal running `terraform apply` (the CI/WIF service account). GCP
# requires this principal to have actAs (roles/iam.serviceAccountUser) on
# runtime SAs to attach them to Cloud Run services/jobs and Scheduler OAuth.
# Format: "serviceAccount:<email>". Empty => skip the binding (e.g. when a
# human with broad IAM applies locally and the binding is unneeded).
variable "deployer_principal" {
  description = "CI/WIF principal that runs terraform apply; granted actAs on the api/worker runtime SAs. Format: serviceAccount:<email>. Empty => no binding."
  type        = string
  default     = ""
}
