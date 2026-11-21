locals {
  # OIDC audience for the maintenance sweep. Deliberately a synthetic constant
  # rather than module.service.uri: feeding the service's own URL back into its
  # env_vars would be a dependency cycle. Cloud Scheduler mints the token with
  # whatever audience it is given, and the API compares it verbatim, so the only
  # requirement is that both sides agree.
  maintenance_oidc_audience = "https://${var.name}/internal/maintenance/sweep"
}

module "service" {
  source = "../../modules/cloud_run_service"

  project_id            = var.project_id
  region                = var.region
  name                  = var.name
  image                 = var.image
  service_account_email = var.service_account_email
  allow_unauthenticated = true
  deletion_protection   = var.deletion_protection

  # One number drives both the Cloud Run hard cancel and the maintenance
  # scheduler's attempt deadline, so a retry can never overlap a sweep that is
  # still running.
  timeout = "${var.request_timeout_seconds}s"

  # PORT is a Cloud Run reserved env var: it is injected automatically from
  # the container port (cloud_run_service container_port, default 8080) and
  # must not be set explicitly. The app reads PORT with an 8080 fallback.
  env_vars = {
    SERVICE_MODE    = "api"
    ENV             = var.env
    WORKER_BASE_URL = var.worker_base_url
    # Cloud Tasks dispatch. When WORKER_CLOUDTASKS_QUEUE is non-empty the
    # API skips its synchronous Connect dispatcher and enqueues onto the
    # queue instead; Cloud Tasks then pushes the task to WORKER_DISPATCH_URL
    # using an OIDC token minted for WORKER_INVOKER_SA.
    WORKER_CLOUDTASKS_QUEUE = var.worker_cloudtasks_queue
    WORKER_DISPATCH_URL     = var.worker_dispatch_url
    WORKER_INVOKER_SA       = var.worker_invoker_sa
    # >= the worker request timeout; set per-task at enqueue (the queue resource
    # has no dispatch_deadline). Same source as the worker timeout (locals).
    WORKER_DISPATCH_DEADLINE_SECONDS = tostring(var.worker_dispatch_deadline_seconds)
    # Base URL is scheme+host only (no bucket path). ValidateBaseURL rejects
    # a path, and BuildDocumentUploadURL inserts the bucket from GCS_BUCKET.
    GCS_BUCKET                   = var.uploads_bucket_name
    GCS_UPLOAD_URL_BASE          = "https://storage.googleapis.com"
    INTERNAL_GCS_UPLOAD_URL_BASE = "https://storage.googleapis.com"
    FIREBASE_PROJECT_ID          = var.firebase_project_id
    CORS_ALLOWED_ORIGINS         = var.cors_allowed_origins
    GEMINI_MODEL                 = var.gemini_model

    # Stripe (non-secret IDs)
    STRIPE_PRO_PRICE_ID       = var.stripe_pro_price_id
    STRIPE_PRO_PRICE_ID_JPY   = var.stripe_pro_price_id_jpy
    STRIPE_PRO_PRICE_ID_USD   = var.stripe_pro_price_id_usd
    STRIPE_DEFAULT_CURRENCY   = var.stripe_default_currency
    STRIPE_METER_INPUT_EVENT  = var.stripe_meter_input_event
    STRIPE_METER_OUTPUT_EVENT = var.stripe_meter_output_event

    # Billing redirect URLs
    BILLING_SUCCESS_URL       = var.billing_success_url
    BILLING_CANCEL_URL        = var.billing_cancel_url
    BILLING_PORTAL_RETURN_URL = var.billing_portal_return_url

    NEW_RELIC_APP_NAME = var.new_relic_app_name

    # Signed upload URLs (private key omitted => IAM SignBlob path)
    GCS_UPLOAD_ISSUER                 = var.gcs_upload_issuer
    GCS_SIGNING_SERVICE_ACCOUNT_EMAIL = var.gcs_signing_service_account_email
    GCS_SIGNED_URL_TTL_MINUTES        = var.gcs_signed_url_ttl_minutes

    SYNTHIFY_ADMIN_USER_EMAILS = var.admin_user_emails
    # Non-empty => only these emails may reach the API (stage lockdown).
    # Empty => no restriction (prod default).
    SYNTHIFY_ALLOWED_USER_EMAILS = var.allowed_user_emails
    LOG_LLM_PAYLOAD              = var.log_llm_payload

    # Cloud Scheduler-driven housekeeping (auto-resume + stuck-job sweep). Both
    # must be set or the API leaves the endpoint unmounted; it never falls back
    # to an open route. The API is allow_unauthenticated=true, so run.invoker
    # cannot gate this and the OIDC token is verified in-process instead.
    SYNTHIFY_MAINTENANCE_OIDC_AUDIENCE              = local.maintenance_oidc_audience
    SYNTHIFY_MAINTENANCE_SCHEDULER_SERVICE_ACCOUNTS = var.maintenance_scheduler_service_account_email
  }

  sensitive_env_vars = {
    SYNTHIFY_READINESS_KEY         = var.readiness_api_key
    SYNTHIFY_READINESS_MONITOR_KEY = var.readiness_monitor_key
  }

  secret_env_vars = [
    {
      name   = "DATABASE_DSN"
      secret = var.secret_ids["database-dsn"]
    },
    {
      name   = "STRIPE_SECRET_KEY"
      secret = var.secret_ids["stripe-secret-key"]
    },
    {
      name   = "STRIPE_WEBHOOK_SECRET"
      secret = var.secret_ids["stripe-webhook-secret"]
    },
    {
      name   = "NEW_RELIC_LICENSE_KEY"
      secret = var.secret_ids["new-relic-license-key"]
    },
    {
      name   = "INTERNAL_WORKER_TOKEN"
      secret = var.secret_ids["internal-worker-token"]
    },
    # The auth middleware reads SYNTHIFY_INTERNAL_SERVICE_TOKEN, while the
    # worker sends the value it loads from INTERNAL_WORKER_TOKEN. Bind both
    # names to the same secret so worker->API service calls authenticate.
    {
      name   = "SYNTHIFY_INTERNAL_SERVICE_TOKEN"
      secret = var.secret_ids["internal-worker-token"]
    },
  ]
}

# Periodic housekeeping that used to run as in-process goroutines in the API
# (auto-resume of failed jobs, and the stuck-job sweep that feeds the New Relic
# JobStuck alert). Timers inside the container only fire when Cloud Run keeps CPU
# allocated between requests — i.e. when the service is billed as always-on — and
# with min-instances=0 the instance is frequently not alive at all. Cloud
# Scheduler drives them instead, so the service can run cpu_idle=true and scale
# to zero.
resource "google_cloud_scheduler_job" "maintenance_sweep" {
  project     = var.project_id
  region      = var.region
  name        = "${var.name}-maintenance-sweep"
  description = "Runs the Synthify API job auto-resume and stuck-job sweeps."
  schedule    = var.maintenance_schedule
  time_zone   = var.maintenance_time_zone

  # Matches the Cloud Run request timeout: a longer deadline would let Scheduler
  # give up while the sweep is still running, and a retry would then overlap it.
  attempt_deadline = "${var.request_timeout_seconds}s"

  retry_config {
    # One retry covers a cold start or a blip. Beyond that the next scheduled
    # run is the retry — the sweeps are idempotent, so nothing is lost by
    # waiting, and hammering a struggling database helps no one.
    retry_count = 1
  }

  http_target {
    http_method = "POST"
    uri         = "${module.service.uri}/internal/maintenance/sweep"

    oidc_token {
      service_account_email = var.maintenance_scheduler_service_account_email
      audience              = local.maintenance_oidc_audience
    }
  }
}
