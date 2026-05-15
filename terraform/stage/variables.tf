variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "environment" {
  type    = string
  default = "stage"
}

variable "api_image" {
  type = string
}

variable "worker_image" {
  type = string
}

variable "uploads_bucket_name" {
  type = string
}

variable "firebase_project_id" {
  type = string
}

variable "cors_allowed_origins" {
  type = string
}

variable "gemini_model" {
  type    = string
  default = "gemini-3-flash-preview"
}

variable "env" {
  type    = string
  default = "stage"
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
  default = "synthify-api-stage"
}

variable "api_base_url" {
  description = "Public URL for the API service. Apply once with default empty, then set to the api Cloud Run URL and re-apply so the worker can call back."
  type        = string
  default     = ""
}
