resource "google_artifact_registry_repository" "repo" {
  location      = var.location
  repository_id = var.repository_id
  description   = var.description
  format        = "DOCKER"

  docker_config {
    immutable_tags = false
  }
}

output "name" {
  value = google_artifact_registry_repository.repo.name
}

output "repository_url" {
  value = "${var.location}-docker.pkg.dev/${var.project_id}/${var.repository_id}"
}
