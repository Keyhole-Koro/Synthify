resource "google_cloud_tasks_queue" "this" {
  project  = var.project_id
  location = var.location
  name     = var.name

  rate_limits {
    max_concurrent_dispatches = var.max_concurrent_dispatches
    max_dispatches_per_second = var.max_dispatches_per_second
  }

  # Cloud Tasks's at-least-once delivery + automatic exponential backoff is
  # the whole reason we use a queue here. Tuning these lets us tolerate
  # worker cold starts (Cloud Run min=0) and transient 5xx without rolling
  # our own retry loop in the API.
  retry_config {
    max_attempts       = var.max_attempts
    min_backoff        = var.min_backoff
    max_backoff        = var.max_backoff
    max_retry_duration = var.max_retry_duration
    max_doublings      = var.max_doublings
  }
}
