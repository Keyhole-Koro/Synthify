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

variable "database_url_secret" {
  type = string
}

variable "worker_token_secret" {
  type = string
}

variable "gemini_api_key_secret" {
  type = string
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
