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
