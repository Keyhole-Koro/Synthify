# Job failure spike: the worker emits a JobFailed custom event whenever
# document processing fails. This is a core-product failure that the HTTP
# readiness cron cannot see.
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

# Stuck job spike: the API periodically sweeps for jobs wedged in QUEUED/RUNNING
# past a grace period and emits a JobStuck event per job_id. uniqueCount(job_id)
# dedups the periodic re-emissions and multiple API instances.
resource "newrelic_nrql_alert_condition" "stuck_jobs" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.critical.id
  type                         = "static"
  name                         = "Stuck job spike"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT uniqueCount(job_id)
      FROM JobStuck
    NRQL
  }

  critical {
    operator              = "above"
    threshold             = var.stuck_job_count_threshold
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}

# Job-notification failure: a dropped status write leaves the UI hanging on a
# finished job — a silent, user-visible failure.
resource "newrelic_nrql_alert_condition" "notification_failures" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.warning.id
  type                         = "static"
  name                         = "Job notification failure spike"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT count(*)
      FROM NotificationFailed
    NRQL
  }

  warning {
    operator              = "above"
    threshold             = var.notification_failure_count_threshold
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}
