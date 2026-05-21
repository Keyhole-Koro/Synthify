# The bootstrap module is everything that must exist BEFORE the Cloud Run
# services can be applied: the platform (SAs, buckets, secrets, registry,
# queues) and every IAM grant the deployer needs. The CD pipeline applies
# this with a single `-target=module.bootstrap` before pushing images, so
# adding an IAM binding here never requires touching the workflow again.

module "platform" {
  source = "../platform"

  project_id          = var.project_id
  region              = var.region
  environment         = var.environment
  uploads_bucket_name = var.uploads_bucket_name
}

# Signed upload URLs use IAM SignBlob when no private key is provided
# (see apps/api/internal/bootstrap/bootstrap.go signBytesWithIAM). The API service
# account must be able to sign blobs as itself.
resource "google_service_account_iam_member" "api_self_sign_blob" {
  count              = var.gcs_upload_issuer == "signed" ? 1 : 0
  service_account_id = "projects/${var.project_id}/serviceAccounts/${module.platform.api_service_account_email}"
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${module.platform.api_service_account_email}"
}

# The principal running `terraform apply` (CI WIF SA) must be able to actAs
# the runtime SAs; GCP enforces this when attaching a SA to a Cloud Run
# service/job or as a Scheduler OAuth identity. Empty deployer_principal =>
# skip (local apply by a broad-IAM human).
locals {
  deployer_acts_as_sas = var.deployer_principal == "" ? {} : {
    worker         = module.platform.worker_service_account_email
    eval           = module.platform.eval_service_account_email
    eval_scheduler = module.platform.eval_scheduler_service_account_email
    api            = module.platform.api_service_account_email
  }
}

resource "google_service_account_iam_member" "deployer_acts_as" {
  for_each           = local.deployer_acts_as_sas
  service_account_id = "projects/${var.project_id}/serviceAccounts/${each.value}"
  role               = "roles/iam.serviceAccountUser"
  member             = var.deployer_principal
}

# Project-level roles for the deployer SA. These were granted out-of-band
# before; bringing them under Terraform makes apply self-bootstrapping and
# keeps stage/prod identical (prod was missing cloudscheduler.admin, which is
# what caused the cloudscheduler.jobs.create 403).
#
# google_project_iam_member is non-authoritative and idempotent: adding a
# binding that already exists (the 11 hand-granted ones on stage) is a no-op,
# and missing ones (cloudscheduler/firebasehosting on prod) are created. No
# import block is used on purpose — import requires the remote binding to
# already exist, which would hard-fail prod where two of these are absent.
#
# Bootstrap note: resourcemanager.projectIamAdmin (already present on both
# deployers out-of-band) is what authorizes the deployer to create these
# bindings. Where the deployer grants itself a role it then depends on in the
# same apply, IAM propagation can lag (the original 403); applying this whole
# module via -target before the dependent resources reconciles IAM first.
resource "google_project_iam_member" "deployer_project_roles" {
  for_each = var.deployer_principal == "" ? toset([]) : toset(var.deployer_project_roles)
  project  = var.project_id
  role     = each.value
  member   = var.deployer_principal
}
