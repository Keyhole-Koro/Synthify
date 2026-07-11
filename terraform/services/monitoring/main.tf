resource "newrelic_alert_policy" "critical" {
  account_id          = var.new_relic_account_id
  name                = "synthify-critical-${var.environment}"
  incident_preference = "PER_CONDITION_AND_TARGET"
}

resource "newrelic_alert_policy" "warning" {
  account_id          = var.new_relic_account_id
  name                = "synthify-warning-${var.environment}"
  incident_preference = "PER_CONDITION_AND_TARGET"
}

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

# Readiness/DB connectivity failures are checked out-of-band via a GitHub
# Actions cron hitting /health?ready=1 (see .github/workflows), not through
# NR Synthetics — Synthetics billing was an unknown at design time and the
# GitHub Actions cron reuses the existing notify-discord workflow at near
# zero marginal cost.

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

# Frontend JS error spike.
resource "newrelic_nrql_alert_condition" "browser_js_errors" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.warning.id
  type                         = "static"
  name                         = "Browser JS error spike"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT count(*)
      FROM JavaScriptError
      WHERE appName = '${var.browser_app_name}'
    NRQL
  }

  warning {
    operator              = "above"
    threshold             = var.js_error_count_threshold
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}
