# Readiness/DB connectivity failures are checked out-of-band via a GitHub
# Actions cron hitting /health?ready=1 (see .github/workflows), not through
# NR Synthetics.

# 5xx error rate: backend API returning server errors at an elevated rate.
resource "newrelic_nrql_alert_condition" "api_error_rate" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.critical.id
  type                         = "static"
  name                         = "API 5xx error rate"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT percentage(count(*), WHERE http.statusCode >= 500)
      FROM Transaction
      WHERE appName = '${var.api_app_name}'
    NRQL
  }

  critical {
    operator              = "above"
    threshold             = var.error_rate_threshold_percent
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}

# Response time: p95 transaction duration creeping up is an early warning
# sign before it becomes a full outage.
resource "newrelic_nrql_alert_condition" "api_response_time" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.warning.id
  type                         = "static"
  name                         = "API p95 response time"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT percentile(duration, 95)
      FROM Transaction
      WHERE appName = '${var.api_app_name}'
    NRQL
  }

  warning {
    operator              = "above"
    threshold             = var.response_time_threshold_seconds
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}

# Auth error spike: a burst of 401/403 on the API suggests credential-stuffing,
# a broken client, or an auth-config regression. Some baseline is normal, so the
# default threshold is deliberately high.
resource "newrelic_nrql_alert_condition" "auth_errors" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.warning.id
  type                         = "static"
  name                         = "API auth error spike"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT count(*)
      FROM Transaction
      WHERE appName = '${var.api_app_name}'
        AND http.statusCode IN (401, 403)
    NRQL
  }

  warning {
    operator              = "above"
    threshold             = var.auth_error_count_threshold
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}
