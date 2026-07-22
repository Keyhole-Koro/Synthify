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

# Optional custom domain (e.g. monitor.synthify.keyhole.work). Prerequisites,
# done out-of-band: (1) verify the domain in this GCP project, and (2) add the
# DNS records Cloud Run returns (see the monitor_dns_records output) to the
# keyhole.work zone. Region availability for domain mappings is limited; if the
# deploy region is unsupported, front the service with a global external HTTPS
# load balancer (serverless NEG) instead and leave var.domain empty.
resource "google_cloud_run_domain_mapping" "this" {
  count    = var.domain == "" ? 0 : 1
  project  = var.project_id
  location = var.region
  name     = var.domain

  metadata {
    namespace = var.project_id
  }

  spec {
    route_name = module.service.name
  }
}
