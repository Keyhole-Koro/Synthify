resource "google_cloud_tasks_queue" "this" {
  project  = var.project_id
  location = var.location
  name     = var.name

  rate_limits {
    max_concurrent_dispatches = var.max_concurrent_dispatches
    max_dispatches_per_second = var.max_dispatches_per_second
  }
}
