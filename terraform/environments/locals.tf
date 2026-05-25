# Derived values. Keep tfvars limited to things that genuinely vary per
# environment and can't be computed (project_id, web_base_url, environment).
# Everything reconstructible is centralized here. Each derived value still
# allows an explicit override via its tfvars variable (empty => derive).

locals {
  # Runtime ENV string. prod => "production"; every other env passes through.
  env = var.environment == "prod" ? "production" : var.environment

  # GCS uploads bucket: <project>-synthify-uploads-<env>.
  uploads_bucket_name = (
    var.uploads_bucket_name != "" ? var.uploads_bucket_name :
    "${var.project_id}-synthify-uploads-${var.environment}"
  )

  # Firebase project defaults to the GCP project unless overridden.
  firebase_project_id = (
    var.firebase_project_id != "" ? var.firebase_project_id : var.project_id
  )

  # New Relic app name: prod stays "synthify-api", others suffixed.
  new_relic_app_name = (
    var.new_relic_app_name != "" ? var.new_relic_app_name :
    var.environment == "prod" ? "synthify-api" : "synthify-api-${var.environment}"
  )

  new_relic_worker_app_name = (
    var.new_relic_worker_app_name != "" ? var.new_relic_worker_app_name :
    var.environment == "prod" ? "synthify-worker" : "synthify-worker-${var.environment}"
  )

  # Cloud Run delete guard: prod is protected; non-prod is replaceable so a
  # tainted/failed revision can be recreated without manual intervention.
  deletion_protection = var.environment == "prod"

  # Web origin drives CORS + billing redirect URLs. Trim any trailing slash
  # so concatenation below is predictable.
  web_base_url = trimsuffix(var.web_base_url, "/")

  default_cors_allowed_origins = join(",", distinct([
    local.web_base_url,
    "https://${var.project_id}.web.app",
    "https://${var.project_id}.firebaseapp.com",
  ]))

  cors_allowed_origins = (
    var.cors_allowed_origins != "" ? var.cors_allowed_origins : local.default_cors_allowed_origins
  )

  billing_success_url = (
    var.billing_success_url != "" ? var.billing_success_url :
    "${local.web_base_url}/workspaces?billing=success"
  )
  billing_cancel_url = (
    var.billing_cancel_url != "" ? var.billing_cancel_url :
    "${local.web_base_url}/workspaces?billing=cancel"
  )
  billing_portal_return_url = (
    var.billing_portal_return_url != "" ? var.billing_portal_return_url :
    "${local.web_base_url}/workspaces"
  )
}
