resource "google_storage_bucket" "this" {
  project       = var.project_id
  name          = var.name
  location      = var.location
  force_destroy = var.force_destroy

  uniform_bucket_level_access = true

  # Nothing in this bucket is public: browsers upload and download through
  # signed URLs, and the services use their own service accounts. Enforcing
  # this makes a grant to allUsers or allAuthenticatedUsers fail instead of
  # publishing every user's files.
  public_access_prevention = "enforced"

  # User uploads live here. Keep prior versions so an accidental overwrite
  # or delete is recoverable. (No prevent_destroy: the managed `down` flow
  # in scripts/manage-env.sh intentionally tears the bucket down, and a
  # static lifecycle block can't be toggled per-environment.)
  versioning {
    enabled = var.versioning
  }

  # ...but not forever. Without this, a file a user deleted stays as a
  # noncurrent version indefinitely, and every overwritten worker checkpoint
  # (.checkpoints/) is kept too. with_state = "ARCHIVED" (isLive = false)
  # limits the rule to noncurrent versions, so a live object is never
  # deleted, however old. Soft delete then keeps a deleted version for its
  # retention period (7 days by default) before it is gone.
  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      with_state                 = "ARCHIVED"
      days_since_noncurrent_time = var.noncurrent_version_retention_days
    }
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
