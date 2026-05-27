resource "google_storage_bucket" "this" {
  project       = var.project_id
  name          = var.name
  location      = var.location
  force_destroy = var.force_destroy

  uniform_bucket_level_access = true

  # User uploads live here. Keep prior versions so an accidental overwrite
  # or delete is recoverable. (No prevent_destroy: the managed `down` flow
  # in scripts/manage-env.sh intentionally tears the bucket down, and a
  # static lifecycle block can't be toggled per-environment.)
  versioning {
    enabled = var.versioning
  }

  dynamic "cors" {
    for_each = length(var.cors_origins) == 0 ? [] : [1]
    content {
      origin          = var.cors_origins
      method          = ["PUT", "POST", "GET", "HEAD"]
      response_header = ["Content-Type"]
      max_age_seconds = 3600
    }
  }
}
