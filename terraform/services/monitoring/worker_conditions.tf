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
  # instrumentation dropped) nothing else would catch it.
  expiration_duration            = var.worker_signal_loss_seconds
  open_violation_on_expiration   = true
  close_violations_on_expiration = true

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
