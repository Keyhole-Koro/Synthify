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

variable "worker_error_rate_threshold_percent" {
  description = "Worker 5xx error rate (%) over 5 minutes that opens a critical incident."
  type        = number
  default     = 5
}

variable "worker_response_time_threshold_seconds" {
  description = "Worker p95 transaction duration (seconds) over 5 minutes that opens a warning incident."
  type        = number
  default     = 10
}

variable "worker_signal_loss_seconds" {
  description = "Seconds without any worker telemetry before a loss-of-signal incident opens. Worker is an internal-only Cloud Run service with no readiness cron, so silence is the only sign it has stopped processing."
  type        = number
  default     = 900
}

variable "job_failure_count_threshold" {
  description = "JobFailed custom-event count over 5 minutes that opens a critical incident. Document processing failures are invisible to the HTTP readiness cron."
  type        = number
  default     = 5
}

variable "billing_error_count_threshold" {
  description = "Billing/Stripe error count over 5 minutes that opens a critical incident. Webhook or meter failures desync subscription state and usage billing."
  type        = number
  default     = 3
}

variable "llm_throttle_count_threshold" {
  description = "LLMThrottled(reason=rate_limited) count over 5 minutes that opens a warning incident. Sustained rate-limiting is the leading indicator of quota starvation, ahead of the job failures it eventually causes."
  type        = number
  default     = 30
}

variable "upload_rejected_count_threshold" {
  description = "UploadRejected count over 5 minutes that opens a warning incident. A spike suggests abuse, a misbehaving client, or a regression in upload validation."
  type        = number
  default     = 50
}

variable "stuck_job_count_threshold" {
  description = "Distinct stuck jobs (JobStuck, by job_id) over 5 minutes before a critical incident opens. A job wedged in QUEUED/RUNNING never emits JobFailed, so this is the only signal it silently stopped progressing. Default 0 = any stuck job alerts."
  type        = number
  default     = 0
}

variable "notification_failure_count_threshold" {
  description = "NotificationFailed count over 5 minutes that opens a warning incident. A dropped Firestore status write leaves the frontend hanging on an otherwise-finished job."
  type        = number
  default     = 3
}

variable "orphan_delete_failure_count_threshold" {
  description = "OrphanObjectDeleteFailed count over 5 minutes that opens a warning incident. Repeated GCS cleanup failures leak storage and cost."
  type        = number
  default     = 5
}

variable "auth_error_count_threshold" {
  description = "API 401/403 count over 5 minutes that opens a warning incident. A spike suggests credential-stuffing, a broken client, or an auth-config regression."
  type        = number
  default     = 100
}
