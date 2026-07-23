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
