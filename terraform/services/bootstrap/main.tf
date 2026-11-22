# The bootstrap module is everything that must exist BEFORE the Cloud Run
# services can be applied: the platform (buckets, secrets, registry, queues).
# The CD pipeline applies this with a single `-target=module.bootstrap` before
# pushing images.
#
# Service accounts and IAM grants are not here: the deploy account cannot set
# project IAM or create service accounts, so they live in terraform/manual/iam
# and a project owner applies them by hand, before the first deploy.

module "platform" {
  source = "../platform"

  pipeline_max_concurrent_dispatches = var.pipeline_max_concurrent_dispatches

  project_id                  = var.project_id
  region                      = var.region
  environment                 = var.environment
  uploads_bucket_name         = var.uploads_bucket_name
  uploads_bucket_cors_origins = var.uploads_bucket_cors_origins
}

# Moved to terraform/manual/iam with the rest of the IAM. These blocks make CD
# drop them from its state without destroying them, so the grants stay while
# the manual stack takes them over. They can go once every environment has
# deployed this.
removed {
  from = google_service_account_iam_member.api_self_sign_blob
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_service_account_iam_member.deployer_acts_as
  lifecycle {
    destroy = false
  }
}

removed {
  from = google_project_iam_member.deployer_project_roles
  lifecycle {
    destroy = false
  }
}
