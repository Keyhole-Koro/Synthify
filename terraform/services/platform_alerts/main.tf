# Cloud Monitoring alerts that do not depend on the app.
#
# The New Relic conditions (services/monitoring) only see what the app's agent
# reports, and they only reach Discord. When the API cannot start, Firebase
# Hosting or the domain breaks, or the Discord webhook stops working, they stay
# silent. These checks probe from outside, read GCP's own audit log, and send
# email, so they are a second path that does not share those failure modes.
# What each alert means and what to do: docs/alerts.md and docs/runbooks/.
locals {
  name_prefix = "Synthify ${upper(var.environment)}"

  web_host = trimsuffix(trimprefix(var.web_base_url, "https://"), "/")
  api_host = trimsuffix(trimprefix(var.api_base_url, "https://"), "/")

  # The address is a secret (the repository is public), so only whether it is
  # set may decide the count.
  alert_email_enabled = nonsensitive(var.alert_email != "")
  send_readiness_key  = nonsensitive(var.readiness_monitor_key != "")

  channels = google_monitoring_notification_channel.email[*].name

  uptime_checks = {
    web = {
      check_id = google_monitoring_uptime_check_config.web.uptime_check_id
      url      = "https://${local.web_host}/"
    }
    api = {
      check_id = google_monitoring_uptime_check_config.api.uptime_check_id
      url      = "https://${local.api_host}/health?ready=1"
    }
  }
}

resource "google_monitoring_notification_channel" "email" {
  count = local.alert_email_enabled ? 1 : 0

  project      = var.project_id
  display_name = "${local.name_prefix} platform alerts"
  type         = "email"
  labels = {
    email_address = var.alert_email
  }
}

# The public page on Firebase Hosting. The title is in the static HTML, so a
# Hosting error page or someone else's site answering 200 still fails.
resource "google_monitoring_uptime_check_config" "web" {
  project      = var.project_id
  display_name = "${local.name_prefix}: web /"
  period       = "300s"
  timeout      = "10s"

  # Three locations, so "two or more failing" below is a majority. Every
  # location probes every period, and each API probe also pings the database.
  selected_regions = var.uptime_check_regions

  http_check {
    path           = "/"
    port           = 443
    use_ssl        = true
    validate_ssl   = true
    request_method = "GET"

    accepted_response_status_codes {
      status_class = "STATUS_CLASS_2XX"
    }
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project_id
      host       = local.web_host
    }
  }

  content_matchers {
    content = "<title>Synthify</title>"
    matcher = "CONTAINS_STRING"
  }

  # An alert policy refers to the check by id, and the API refuses to delete a
  # check that a policy still uses. Create the new one first, move the policy
  # over, then delete the old one (a host change replaces the check).
  lifecycle {
    create_before_destroy = true
  }
}

# Changes whenever the readiness key changes, without keeping the key itself.
# The API check below is replaced when this changes; see its lifecycle block.
resource "terraform_data" "readiness_key_version" {
  input = sha256(var.readiness_monitor_key)
}

# The API's readiness endpoint: 200 "ok" only when the API can reach the
# database. It needs the long-lived monitor key in X-Synthify-Readiness-Key
# (apps/api/internal/bootstrap/application.go healthHandler); without it the
# API answers 401 and this check fails.
resource "google_monitoring_uptime_check_config" "api" {
  project      = var.project_id
  display_name = "${local.name_prefix}: api /health?ready=1"
  period       = "300s"
  # A cold start pays for container start-up and the first database connection
  # (CockroachDB Serverless may be resuming too). A warm answer takes well
  # under a second; this check is for outages, not latency.
  timeout          = "30s"
  selected_regions = var.uptime_check_regions

  http_check {
    path           = "/health?ready=1"
    port           = 443
    use_ssl        = true
    validate_ssl   = true
    request_method = "GET"
    headers        = local.send_readiness_key ? { "X-Synthify-Readiness-Key" = var.readiness_monitor_key } : null
    # Get and List return the header as "******", so the key is not readable
    # from the uptime check configuration.
    mask_headers = local.send_readiness_key

    accepted_response_status_codes {
      status_class = "STATUS_CLASS_2XX"
    }
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project_id
      host       = local.api_host
    }
  }

  content_matchers {
    content = "^ok$"
    matcher = "MATCHES_REGEX"
  }

  lifecycle {
    create_before_destroy = true
    # The API returns masked headers, so Terraform would see a change on every
    # apply. Ignore them, and replace the check when the key itself changes.
    ignore_changes       = [http_check[0].headers]
    replace_triggered_by = [terraform_data.readiness_key_version]
  }
}

# Fires when two or more locations fail for 10 minutes (two periods). One
# location failing on its own is usually that location's network.
resource "google_monitoring_alert_policy" "uptime" {
  for_each = local.uptime_checks

  project               = var.project_id
  display_name          = "${local.name_prefix}: ${each.key} unreachable"
  combiner              = "OR"
  severity              = "CRITICAL"
  notification_channels = local.channels

  conditions {
    display_name = "${each.key} uptime check failing in 2+ locations"

    condition_threshold {
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" AND metric.label.check_id=\"${each.value.check_id}\" AND resource.type=\"uptime_url\""
      comparison      = "COMPARISON_GT"
      threshold_value = 1
      duration        = "600s"

      aggregations {
        alignment_period     = "300s"
        per_series_aligner   = "ALIGN_NEXT_OLDER"
        cross_series_reducer = "REDUCE_COUNT_FALSE"
        group_by_fields      = ["resource.label.host"]
      }

      trigger {
        count = 1
      }
    }
  }

  alert_strategy {
    auto_close = "1800s"
  }

  documentation {
    mime_type = "text/markdown"
    content   = "${each.value.url} does not answer from outside. Runbook: ${var.runbook_base_url}/uptime.md"
  }
}

# Admin changes made outside CD. The Admin Activity audit log is always on and
# cannot be excluded, so this works without any logging setup. One matching
# entry opens an incident. Watched:
# - IAM policy changes: every method whose name contains SetIamPolicy (project,
#   service accounts, secrets, Cloud Run, Artifact Registry, ...). The ":"
#   operator is case-insensitive, so service accounts' SetIAMPolicy matches
#   too. Buckets use storage.setIamPermissions.
# - Service account keys created or uploaded.
# - Cloud Run services and jobs created, updated or deleted, through the v1
#   API (updates are ReplaceService / ReplaceJob) and the v2 API (what
#   Terraform's google_cloud_run_v2_* resources use).
# - Secret Manager secrets created and versions (values) added.
# - Workload Identity providers created or updated. Loosening a provider's
#   condition lets other workflows become the deploy service account, whose
#   later changes this policy does not see.
# Not alerted: the deploy service account (every deploy does all of the
# above), Google-managed service agents (service-<number>@gcp-sa-*, and
# <number>@cloudservices), and Google's internal principals
# (*@system.gserviceaccount.com, e.g. grants made when an API is enabled).
# The exclusions use NOT, so an entry without a principal still alerts.
resource "google_monitoring_alert_policy" "admin_changes" {
  project               = var.project_id
  display_name          = "${local.name_prefix}: admin change outside CD"
  combiner              = "OR"
  severity              = "CRITICAL"
  notification_channels = local.channels

  conditions {
    display_name = "IAM, service account key, Cloud Run, Secret Manager or Workload Identity change outside CD"

    condition_matched_log {
      filter = <<-EOT
        logName="projects/${var.project_id}/logs/cloudaudit.googleapis.com%2Factivity"
        (
          protoPayload.methodName:"SetIamPolicy"
          OR protoPayload.methodName="storage.setIamPermissions"
          OR protoPayload.methodName="google.iam.admin.v1.CreateServiceAccountKey"
          OR protoPayload.methodName="google.iam.admin.v1.UploadServiceAccountKey"
          OR protoPayload.methodName="google.cloud.run.v1.Services.CreateService"
          OR protoPayload.methodName="google.cloud.run.v1.Services.ReplaceService"
          OR protoPayload.methodName="google.cloud.run.v1.Services.DeleteService"
          OR protoPayload.methodName="google.cloud.run.v2.Services.CreateService"
          OR protoPayload.methodName="google.cloud.run.v2.Services.UpdateService"
          OR protoPayload.methodName="google.cloud.run.v2.Services.DeleteService"
          OR protoPayload.methodName="google.cloud.run.v1.Jobs.CreateJob"
          OR protoPayload.methodName="google.cloud.run.v1.Jobs.ReplaceJob"
          OR protoPayload.methodName="google.cloud.run.v1.Jobs.DeleteJob"
          OR protoPayload.methodName="google.cloud.run.v2.Jobs.CreateJob"
          OR protoPayload.methodName="google.cloud.run.v2.Jobs.UpdateJob"
          OR protoPayload.methodName="google.cloud.run.v2.Jobs.DeleteJob"
          OR protoPayload.methodName="google.cloud.secretmanager.v1.SecretManagerService.CreateSecret"
          OR protoPayload.methodName="google.cloud.secretmanager.v1.SecretManagerService.AddSecretVersion"
          OR protoPayload.methodName="google.iam.v1.WorkloadIdentityPools.CreateWorkloadIdentityPoolProvider"
          OR protoPayload.methodName="google.iam.v1.WorkloadIdentityPools.UpdateWorkloadIdentityPoolProvider"
        )
        %{if var.deploy_service_account_email != ""}NOT protoPayload.authenticationInfo.principalEmail="${var.deploy_service_account_email}"%{endif}
        NOT protoPayload.authenticationInfo.principalEmail=~"^service-[0-9]+@gcp-sa-[a-z0-9-]+\.iam\.gserviceaccount\.com$"
        NOT protoPayload.authenticationInfo.principalEmail=~"^[0-9]+@cloudservices\.gserviceaccount\.com$"
        NOT protoPayload.authenticationInfo.principalEmail=~"@system\.gserviceaccount\.com$"
      EOT

      label_extractors = {
        principal     = "EXTRACT(protoPayload.authenticationInfo.principalEmail)"
        method        = "EXTRACT(protoPayload.methodName)"
        resource_name = "EXTRACT(protoPayload.resourceName)"
      }
    }
  }

  alert_strategy {
    auto_close = "1800s"

    # Required for log-based policies. At most one notification per 5 minutes
    # for the same principal, method and resource.
    notification_rate_limit {
      period = "300s"
    }
  }

  documentation {
    mime_type = "text/markdown"
    content   = "$${log.extracted_label.principal} ran $${log.extracted_label.method} on $${log.extracted_label.resource_name}, outside CD. Runbook: ${var.runbook_base_url}/cloud-admin-changes.md"
  }
}
