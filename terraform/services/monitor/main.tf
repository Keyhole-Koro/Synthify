# The monitor dashboard (Next.js BFF). It reads Postgres directly through a
# read-only `monitor` role and serves the operations dashboards. Access is
# gated at the app layer (requireAdmin: Firebase ID token + admin allowlist),
# so the network ingress is public like the API.
module "service" {
  source = "../../modules/cloud_run_service"

  project_id            = var.project_id
  region                = var.region
  name                  = var.name
  image                 = var.image
  service_account_email = var.service_account_email

  # Public ingress; the app enforces admin-only access via requireAdmin. A
  # signed-in non-admin (or anonymous) request is rejected with 401/403 by
  # every /api/* route, and the UI shows a sign-in gate.
  allow_unauthenticated = true
  ingress               = "INGRESS_TRAFFIC_ALL"
  deletion_protection   = var.deletion_protection

  # PORT is a Cloud Run reserved env var, injected from the container port
  # (default 8080). The Next.js standalone server reads PORT/HOSTNAME.
  env_vars = {
    NODE_ENV = "production"
    HOSTNAME = "0.0.0.0"
    # Firebase ID token verification (project id + Google public certs; no
    # service-account key needed, same as the API's authenticator).
    FIREBASE_PROJECT_ID = var.firebase_project_id
    # Admin allowlist shared with the API. Empty => fail-closed (all 403).
    SYNTHIFY_ADMIN_USER_EMAILS = var.admin_user_emails
  }

  # Read-only DSN for the `monitor` Postgres role. Distinct from the api/worker
  # database-dsn secret so the dashboard can only SELECT.
  secret_env_vars = [
    {
      name   = "MONITOR_DATABASE_URL"
      secret = var.secret_ids["monitor-database-dsn"]
    },
  ]
}
