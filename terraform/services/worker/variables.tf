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

variable "deletion_protection" {
  description = "Cloud Run delete guard. false in non-prod so a tainted service can be replaced."
  type        = bool
  default     = true
}
