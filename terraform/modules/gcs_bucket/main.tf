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
}
