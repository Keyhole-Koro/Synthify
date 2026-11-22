variable "project_id" {
  type = string
}

variable "environment" {
  description = "stage or prod. Also the GitHub environment and the branch the deploy must run from."
  type        = string

  validation {
    condition     = contains(["stage", "prod"], var.environment)
    error_message = "environment is stage or prod."
  }
}

variable "github_repository" {
  description = "owner/name of the repository whose deploy workflows may act as the deploy account."
  type        = string
  default     = "Keyhole-Koro/Synthify"
}

variable "deploy_account_id" {
  description = "Account id of the deploy service account (GCP_WIF_SA_EMAIL is <id>@<project>.iam.gserviceaccount.com)."
  type        = string
  default     = "github-deploy"
}

variable "workload_identity_pool_id" {
  type    = string
  default = "github-actions"
}

variable "workload_identity_provider_id" {
  type    = string
  default = "github"
}

variable "adopt_existing_workload_identity" {
  description = "Import the pool and provider that already exist (stage and prod). false on a new project, where this stack creates them."
  type        = bool
  default     = true
}

# Everything terraform/environments manages, and nothing that sets IAM or
# creates service accounts.
# - datastore.owner: google_firestore_field (TTL on jobs.expiresAt) needs
#   datastore.databases.update, which no narrower curated role has.
# - firebasehosting.admin, firebaserules.admin: deploy-frontend.yml.
# - monitoring.*: services/platform_alerts (alert policies, the email
#   channel, uptime checks).
# - The *.admin roles can still set IAM on their own resources (a bucket, a
#   secret, a Cloud Run service), which CD needs for those bindings; none of
#   them reaches project IAM or service accounts.
variable "deploy_project_roles" {
  description = "Project roles of the deploy account."
  type        = list(string)
  default = [
    "roles/artifactregistry.admin",
    "roles/cloudscheduler.admin",
    "roles/cloudtasks.admin",
    "roles/datastore.owner",
    "roles/firebasehosting.admin",
    "roles/firebaserules.admin",
    "roles/monitoring.alertPolicyEditor",
    "roles/monitoring.notificationChannelEditor",
    "roles/monitoring.uptimeCheckConfigEditor",
    "roles/run.admin",
    "roles/secretmanager.admin",
    "roles/serviceusage.serviceUsageAdmin",
    "roles/storage.admin",
  ]
}
