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

# 5xx error rate on the worker's HTTP surface (Cloud Tasks dispatch and the
# internal Connect RPC endpoints). The worker is the heaviest, most
# failure-prone service (LLM calls, chunking, tree generation) yet had no
# alert wired to its APM entity even though worker_app_name was already
# plumbed into this module.
resource "newrelic_nrql_alert_condition" "worker_error_rate" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.critical.id
  type                         = "static"
  name                         = "Worker 5xx error rate"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT percentage(count(*), WHERE http.statusCode >= 500)
      FROM Transaction
      WHERE appName = '${var.worker_app_name}'
    NRQL
  }

  critical {
    operator              = "above"
    threshold             = var.worker_error_rate_threshold_percent
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  # Loss of signal: unlike the API, the worker is internal-only and has no
  # readiness cron, so if it stops reporting entirely (crash, broken deploy,
  # instrumentation dropped) nothing else would catch it. Open an incident
  # when telemetry goes silent.
  expiration {
    expiration_duration            = var.worker_signal_loss_seconds
    open_violation_on_expiration   = true
    close_violations_on_expiration = true
  }

  fill_option        = "none"
  aggregation_window = 300
}

# Worker p95 processing latency: early-warning that the LLM/tree pipeline is
# degrading before it turns into failed jobs.
resource "newrelic_nrql_alert_condition" "worker_response_time" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.warning.id
  type                         = "static"
  name                         = "Worker p95 response time"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT percentile(duration, 95)
      FROM Transaction
      WHERE appName = '${var.worker_app_name}'
    NRQL
  }

  warning {
    operator              = "above"
    threshold             = var.worker_response_time_threshold_seconds
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}

# Job failure spike: the worker emits a JobFailed custom event whenever
# document processing fails (see internal/platform/job/lifecycle). This is a
# core-product failure that the HTTP readiness cron cannot see — to the user
# it looks like a job that never finishes. Count-based so a burst of failures
# opens a critical incident regardless of overall traffic.
resource "newrelic_nrql_alert_condition" "job_failure_rate" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.critical.id
  type                         = "static"
  name                         = "Job failure spike"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT count(*)
      FROM JobFailed
    NRQL
  }

  critical {
    operator              = "above"
    threshold             = var.job_failure_count_threshold
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}

# Billing/Stripe failure spike: the API tags billing NoticeError events with an
# `event` attribute prefixed `billing.` (checkout, portal, webhook apply, meter
# emission — see billingService.noticeError). These are revenue-impacting:
# a failed webhook desyncs subscription state and a failed meter emission drops
# usage-based charges. Rare in steady state, so a low threshold is intentional.
resource "newrelic_nrql_alert_condition" "billing_errors" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.critical.id
  type                         = "static"
  name                         = "Billing/Stripe error spike"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT count(*)
      FROM TransactionError
      WHERE appName = '${var.api_app_name}'
        AND event LIKE 'billing.%'
    NRQL
  }

  critical {
    operator              = "above"
    threshold             = var.billing_error_count_threshold
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
