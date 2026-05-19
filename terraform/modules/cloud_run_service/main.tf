resource "google_cloud_run_v2_service" "this" {
  project             = var.project_id
  location            = var.region
  name                = var.name
  ingress             = var.ingress
  deletion_protection = var.deletion_protection

  template {
    service_account = var.service_account_email
    timeout         = var.timeout

    # gcsfuse requires the second-generation execution environment.
    execution_environment = var.gcs_volume != null ? "EXECUTION_ENVIRONMENT_GEN2" : null

    scaling {
      min_instance_count = var.min_instance_count
      max_instance_count = var.max_instance_count
    }

    dynamic "volumes" {
      for_each = var.gcs_volume != null ? [var.gcs_volume] : []
      content {
        name = "gcs"
        gcs {
          bucket    = volumes.value.bucket
          read_only = true
        }
      }
    }

    containers {
      image = var.image

      resources {
        limits = {
          cpu    = var.cpu
          memory = var.memory
        }
      }

      ports {
        container_port = var.container_port
      }

      dynamic "volume_mounts" {
        for_each = var.gcs_volume != null ? [var.gcs_volume] : []
        content {
          name       = "gcs"
          mount_path = volume_mounts.value.mount_path
        }
      }

      dynamic "env" {
        for_each = var.env_vars
        content {
          name  = env.key
          value = env.value
        }
      }

      dynamic "env" {
        for_each = nonsensitive(toset(keys(var.sensitive_env_vars)))
        content {
          name  = env.value
          value = var.sensitive_env_vars[env.value]
        }
      }

      dynamic "env" {
        for_each = var.secret_env_vars
        content {
          name = env.value.name
          value_source {
            secret_key_ref {
              secret  = env.value.secret
              version = env.value.version
            }
          }
        }
      }
    }
  }
}

resource "google_cloud_run_v2_service_iam_member" "invoker" {
  count    = var.allow_unauthenticated ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.this.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
