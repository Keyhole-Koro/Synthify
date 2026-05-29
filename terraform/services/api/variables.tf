variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "name" {
  type = string
}

variable "image" {
  type = string
}

variable "service_account_email" {
  type = string
}

variable "worker_base_url" {
  type = string
}

variable "worker_cloudtasks_queue" {
  description = "Cloud Tasks queue path 'projects/X/locations/Y/queues/Z'. Empty falls back to synchronous Connect dispatch."
  type        = string
  default     = ""
}

variable "worker_dispatch_url" {
  description = "Worker URL Cloud Tasks pushes to (the worker's /internal/dispatch-job endpoint)."
  type        = string
  default     = ""
}

variable "worker_invoker_sa" {
  description = "Service account Cloud Tasks impersonates to mint the OIDC token sent to the worker."
  type        = string
  default     = ""
}

variable "uploads_bucket_name" {
  type = string
}

variable "secret_ids" {
  description = "platform module output: map of secret key -> secret_id"
  type        = map(string)
}

variable "firebase_project_id" {
  type = string
}

variable "cors_allowed_origins" {
  type = string
}

variable "gemini_model" {
  type = string
}

variable "env" {
  description = "Runtime ENV value passed to the API binary"
  type        = string
  default     = "production"
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

variable "billing_success_url" {
  type = string
}

variable "billing_cancel_url" {
  type = string
}

variable "billing_portal_return_url" {
  type = string
}

variable "new_relic_app_name" {
  type    = string
  default = "synthify-api"
}

variable "readiness_api_key" {
  type      = string
  default   = ""
  sensitive = true
}

variable "gcs_upload_issuer" {
  type    = string
  default = "signed"
}

variable "gcs_signing_service_account_email" {
  type    = string
  default = ""
}

variable "gcs_signed_url_ttl_minutes" {
  type    = string
  default = "15"
}

variable "admin_user_emails" {
  type    = string
  default = ""
}

variable "allowed_user_emails" {
  description = "Comma-separated allowlist. Non-empty restricts API access to these emails."
  type        = string
  default     = ""
}

variable "log_llm_payload" {
  type    = string
  default = "false"
}

variable "deletion_protection" {
  description = "Cloud Run delete guard. false in non-prod so a tainted service can be replaced."
  type        = bool
  default     = true
}
