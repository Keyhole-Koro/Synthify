module "service" {
  source = "../../modules/cloud_run_service"

  project_id            = var.project_id
  region                = var.region
  name                  = var.name
  image                 = var.image
  service_account_email = var.service_account_email
  allow_unauthenticated = true
  deletion_protection   = var.deletion_protection

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
  }

  sensitive_env_vars = {
    SYNTHIFY_READINESS_KEY = var.readiness_api_key
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
