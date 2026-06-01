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

variable "gemini_model" {
  type = string
}

variable "api_base_url" {
  description = "Base URL the worker uses to call back into the API (usage metering, etc.)"
  type        = string
}

variable "new_relic_app_name" {
  type    = string
  default = "synthify-worker"
}

variable "readiness_api_key" {
  type      = string
  default   = ""
  sensitive = true
}

variable "log_llm_payload" {
  description = "Set to \"true\" to log raw LLM payloads, including raw agent responses (debug only)."
  type        = string
  default     = "false"
}

variable "deletion_protection" {
  description = "Cloud Run delete guard. false in non-prod so a tainted service can be replaced."
  type        = bool
  default     = true
}

variable "request_timeout_seconds" {
  description = <<-EOT
    Single source of truth for the worker request budget. Feeds BOTH the Cloud
    Run request timeout and the app's WORKER_REQUEST_TIMEOUT_SECONDS env var, so
    the app's self-imposed agent budget (90% of this) and Cloud Run's hard
    cancel stay derived from one number. Must be <= the Cloud Tasks queue
    dispatch_deadline, or a retry can fire before the request finishes and
    double-dispatch the job. Default 600s (10 min); Cloud Run max is 3600s.
  EOT
  type        = number
  default     = 600
}
