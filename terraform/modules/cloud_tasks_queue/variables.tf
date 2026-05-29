variable "project_id" {
  type = string
}

variable "location" {
  type = string
}

variable "name" {
  type = string
}

variable "max_concurrent_dispatches" {
  type    = number
  default = 10
}

variable "max_dispatches_per_second" {
  type    = number
  default = 5
}

# Retry tuning for transient delivery failures (Cloud Run cold starts that
# blow past the per-attempt deadline, worker 5xx, etc.). Defaults are
# generous because the worker job is long-running: up to 7 attempts spread
# over an hour, with exponential backoff capped at 5 minutes.
variable "max_attempts" {
  type    = number
  default = 7
}

variable "min_backoff" {
  type    = string
  default = "30s"
}

variable "max_backoff" {
  type    = string
  default = "300s"
}

variable "max_retry_duration" {
  type    = string
  default = "3600s"
}

variable "max_doublings" {
  type    = number
  default = 4
}
