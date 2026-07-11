variable "environment" {
  type = string
}

variable "new_relic_account_id" {
  type = string
}

variable "api_app_name" {
  description = "New Relic APM entity name for the API service."
  type        = string
}

variable "worker_app_name" {
  description = "New Relic APM entity name for the worker service."
  type        = string
}

variable "browser_app_name" {
  description = "New Relic Browser entity name for the web frontend."
  type        = string
}

variable "discord_alert_webhook_url" {
  description = "Discord incoming webhook URL that receives alert notifications. Sent via Slack-compatible payload (Discord accepts Slack-format webhooks)."
  type        = string
  sensitive   = true
}

variable "error_rate_threshold_percent" {
  description = "5xx error rate (%) over 5 minutes that opens a critical incident."
  type        = number
  default     = 5
}

variable "response_time_threshold_seconds" {
  description = "p95 transaction duration (seconds) over 5 minutes that opens a warning incident."
  type        = number
  default     = 3
}

variable "js_error_count_threshold" {
  description = "Browser JS error count over 5 minutes that opens a warning incident."
  type        = number
  default     = 10
}
