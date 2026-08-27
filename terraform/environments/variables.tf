variable "project_id" {
  type = string
}

variable "region" {
  type = string
}

variable "environment" {
  description = "Resource-name suffix and Secret Manager suffix (stage | prod)"
  type        = string
}

variable "api_image" {
  description = "API container image. Passed by CD via -var after build&push; not stored in tfvars."
  type        = string
  default     = ""
}

variable "worker_image" {
  description = "Worker container image. Passed by CD via -var after build&push; not stored in tfvars."
  type        = string
  default     = ""
}

variable "eval_image" {
  description = "Eval Cloud Run Job container image. Passed by CD via -var after build&push; not stored in tfvars."
  type        = string
  default     = ""
}

variable "monitor_image" {
  description = "Monitor dashboard container image. Passed by CD via -var after build&push; not stored in tfvars."
  type        = string
  default     = ""
}

variable "monitor_domain" {
  description = "Custom domain (bare hostname, no scheme) for the monitor dashboard, set per-env in tfvars. Empty => no domain mapping (use the run.app URL)."
  type        = string
  default     = ""
}

# Public web origin (no trailing slash), e.g. https://stage.synthify.keyhole.work.
# CORS + billing redirect URLs are derived from this in locals.tf.
variable "web_base_url" {
  description = "Public web app origin. Drives cors_allowed_origins and billing_* URLs unless those are set explicitly."
  type        = string
}

# The following are derived in locals.tf when left empty. Set them only to
# override the derived value.
variable "uploads_bucket_name" {
  description = "Empty => <project_id>-synthify-uploads-<environment>."
  type        = string
  default     = ""
}

variable "firebase_project_id" {
  description = "Empty => same as project_id."
  type        = string
  default     = ""
}

variable "cors_allowed_origins" {
  description = "Empty => web_base_url plus Firebase Hosting default domains."
  type        = string
  default     = ""
}

variable "gemini_model" {
  type    = string
  default = "gemini-3.5-flash"
}

variable "eval_schedule" {
  description = "Cloud Scheduler cron expression for LLM eval Cloud Run Job."
  type        = string
  default     = "0 4 * * *"
}

variable "eval_time_zone" {
  type    = string
  default = "Asia/Tokyo"
}

variable "maintenance_schedule" {
  description = "Cron expression for the API job auto-resume / stuck-job sweep."
  type        = string
  default     = "*/5 * * * *"
}

variable "maintenance_time_zone" {
  type    = string
  default = "Asia/Tokyo"
}

variable "stripe_pro_price_id" {
  type    = string
  default = ""
}

variable "stripe_pro_price_id_jpy" {
  type    = string
  default = ""
}

variable "stripe_pro_price_id_usd" {
  type    = string
  default = ""
}

variable "stripe_default_currency" {
  type    = string
  default = "jpy"
}

variable "stripe_meter_input_event" {
  type    = string
  default = ""
}

variable "stripe_meter_output_event" {
  type    = string
  default = ""
}

# Empty => derived from web_base_url in locals.tf.
variable "billing_success_url" {
  description = "Empty => <web_base_url>/workspaces?billing=success."
  type        = string
  default     = ""
}

variable "billing_cancel_url" {
  description = "Empty => <web_base_url>/workspaces?billing=cancel."
  type        = string
  default     = ""
}

variable "billing_portal_return_url" {
  description = "Empty => <web_base_url>/workspaces."
  type        = string
  default     = ""
}

variable "new_relic_app_name" {
  description = "Empty => prod: synthify-api, else synthify-api-<environment>."
  type        = string
  default     = ""
}

variable "new_relic_worker_app_name" {
  description = "Empty => prod: synthify-worker, else synthify-worker-<environment>."
  type        = string
  default     = ""
}

variable "new_relic_browser_app_name" {
  description = "New Relic Browser entity name for the web frontend (NEXT_PUBLIC_NEW_RELIC_BROWSER_APPLICATION_ID's app)."
  type        = string
  # Empty default so the Pass-1 (-target=module.bootstrap) apply, which does
  # not pass these vars, doesn't block on an interactive prompt. The real
  # value is supplied via -var in the Pass-2/3 full applies.
  default = ""
}

variable "new_relic_account_id" {
  description = "New Relic account ID, used by the newrelic Terraform provider to manage alert policies."
  type        = string
  default     = ""
}

variable "new_relic_api_key" {
  description = "New Relic User API key (NRAK-...) used by the newrelic Terraform provider."
  type        = string
  sensitive   = true
  default     = ""
}

variable "discord_alert_webhook_url" {
  description = "Discord incoming webhook URL that receives application alert notifications (separate channel from CI/CD alerts)."
  type        = string
  sensitive   = true
  default     = ""
}

variable "api_base_url" {
  description = "Public URL for the API service. Apply once with default empty, then set to the api Cloud Run URL and re-apply so the worker can call back."
  type        = string
  default     = ""
}

variable "readiness_api_key" {
  description = "Ephemeral deploy-smoke key injected into API/Worker Cloud Run revisions for /health?ready=1."
  type        = string
  default     = ""
  sensitive   = true
}

variable "readiness_monitor_key" {
  description = "Long-lived key for external uptime monitoring of /health?ready=1 (GitHub Actions cron), distinct from readiness_api_key which rotates on every deploy."
  type        = string
  default     = ""
  sensitive   = true
}

# --------------------------------------------------------------------
# Signed upload URLs
# --------------------------------------------------------------------

variable "gcs_upload_issuer" {
  description = "'signed' uses real GCS signed URLs (IAM SignBlob). Anything else uses the fake/dev issuer."
  type        = string
  default     = "signed"
}

variable "gcs_signing_service_account_email" {
  description = "SA email used to sign upload URLs. Empty => use the API service account (recommended)."
  type        = string
  default     = ""
}

variable "gcs_signed_url_ttl_minutes" {
  type    = string
  default = "15"
}

variable "admin_user_emails" {
  description = "Comma-separated list of admin user emails (SYNTHIFY_ADMIN_USER_EMAILS)."
  type        = string
  default     = ""
}

# Access allowlist. Non-empty => only these emails may use the API.
# Typically supplied per-env by CD from a GitHub Environment variable
# (empty for prod => unrestricted; set for stage to lock it down).
variable "allowed_user_emails" {
  description = "Comma-separated allowlist (SYNTHIFY_ALLOWED_USER_EMAILS). Empty => no restriction."
  type        = string
  default     = ""
}

variable "log_llm_payload" {
  description = "Set to \"true\" to log raw LLM payloads (debug only)."
  type        = string
  default     = "false"
}

# Principal running `terraform apply` (the CI/WIF service account). GCP
# requires this principal to have actAs (roles/iam.serviceAccountUser) on
# runtime SAs to attach them to Cloud Run services/jobs and Scheduler OAuth.
# Format: "serviceAccount:<email>". Empty => skip the binding (e.g. when a
# human with broad IAM applies locally and the binding is unneeded).
variable "deployer_principal" {
  description = "CI/WIF principal that runs terraform apply; granted actAs on the api/worker runtime SAs. Format: serviceAccount:<email>. Empty => no binding."
  type        = string
  default     = ""
}

# Project-level roles the deployer SA needs to create/manage every resource
# in this config. Previously these were granted out-of-band (gcloud/console),
# so they were non-reproducible: stage worked only because cloudscheduler.admin
# was hand-added after a 403, and prod still lacks it. Managing them here makes
# every environment identical and self-bootstrapping. resourcemanager.projectIamAdmin
# (in this set) is what lets the deployer grant itself the rest.
#
# datastore.owner is required for google_firestore_field (TTL policy on
# jobs.expiresAt). The Firestore Field/TTL API checks datastore.databases.update,
# which is not in datastore.indexAdmin or datastore.user — only owner has it
# among the published curated roles.
variable "deployer_project_roles" {
  description = "Project-level roles bound to deployer_principal so the deployer can manage all resources in this config. Empty deployer_principal => no bindings."
  type        = list(string)
  default = [
    "roles/artifactregistry.admin",
    "roles/cloudscheduler.admin",
    "roles/cloudtasks.admin",
    "roles/datastore.owner",
    "roles/firebasehosting.admin",
    "roles/firebaserules.admin",
    "roles/iam.serviceAccountAdmin",
    "roles/iam.serviceAccountTokenCreator",
    "roles/resourcemanager.projectIamAdmin",
    "roles/run.admin",
    "roles/secretmanager.admin",
    "roles/serviceusage.serviceUsageAdmin",
    "roles/storage.admin",
  ]
}
