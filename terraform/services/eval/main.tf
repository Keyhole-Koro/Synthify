resource "google_cloud_run_v2_job" "this" {
  project  = var.project_id
  location = var.region
  name     = var.name

  template {
    task_count  = 1
    parallelism = 1

    template {
      service_account = var.service_account_email
      timeout         = var.timeout
      max_retries     = 0

      containers {
        image = var.image

        args = [
          "--cases",
          "apps/eval/cases",
          "--format",
          "json",
        ]

        resources {
          limits = {
            cpu    = var.cpu
            memory = var.memory
          }
        }

        env {
          name  = "GEMINI_MODEL"
          value = var.gemini_model
        }

        env {
          name  = "EVAL_OUTPUT_GCS_URI"
          value = var.output_gcs_uri
        }

        env {
          name = "GEMINI_API_KEY"
          value_source {
            secret_key_ref {
              secret  = var.secret_ids["gemini-api-key"]
              version = "latest"
            }
          }
        }
      }
    }
  }
}

resource "google_cloud_run_v2_job_iam_member" "scheduler_runner" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_job.this.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${var.scheduler_service_account_email}"
}

resource "google_cloud_scheduler_job" "this" {
  project     = var.project_id
  region      = var.region
  name        = "${var.name}-schedule"
  description = "Runs Synthify LLM eval cases as a Cloud Run Job."
  schedule    = var.schedule
  time_zone   = var.time_zone

  http_target {
    http_method = "POST"
    uri         = "https://run.googleapis.com/v2/projects/${var.project_id}/locations/${var.region}/jobs/${google_cloud_run_v2_job.this.name}:run"

    oauth_token {
      service_account_email = var.scheduler_service_account_email
    }
  }
}
