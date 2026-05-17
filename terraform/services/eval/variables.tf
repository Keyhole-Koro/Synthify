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

variable "scheduler_service_account_email" {
  type = string
}

variable "secret_ids" {
  description = "platform module output: map of secret key -> secret_id"
  type        = map(string)
}

variable "gemini_model" {
  type = string
}

variable "schedule" {
  description = "Cloud Scheduler cron expression for eval job."
  type        = string
}

variable "time_zone" {
  type    = string
  default = "Asia/Tokyo"
}

variable "timeout" {
  type    = string
  default = "1800s"
}

variable "cpu" {
  type    = string
  default = "1"
}

variable "memory" {
  type    = string
  default = "1Gi"
}
