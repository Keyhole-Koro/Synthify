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

variable "secret_ids" {
  description = "platform module output: map of secret key -> secret_id"
  type        = map(string)
}

variable "firebase_project_id" {
  type = string
}

variable "admin_user_emails" {
  description = "Comma-separated admin allowlist. Only these emails may reach the dashboards/jobs APIs. Empty => fail-closed (all 403)."
  type        = string
  default     = ""
}

variable "deletion_protection" {
  description = "Cloud Run delete guard. false in non-prod so a tainted service can be replaced."
  type        = bool
  default     = true
}

variable "domain" {
  description = "Custom domain to map to the service (e.g. monitor.synthify.keyhole.work). Empty => no mapping (use the run.app URL). The domain must be verified in the project and its DNS records pointed at Cloud Run out-of-band."
  type        = string
  default     = ""
}
