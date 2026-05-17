output "name" {
  value = google_cloud_run_v2_job.this.name
}

output "schedule_name" {
  value = google_cloud_scheduler_job.this.name
}
