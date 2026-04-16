output "name" {
  value = google_storage_bucket.this.name
}

output "url" {
  value = "gs://${google_storage_bucket.this.name}"
}
