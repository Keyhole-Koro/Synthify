# Billing/Stripe failure spike: billing NoticeError events use an `event`
# attribute prefixed with `billing.`. These are revenue-impacting because a
# failed webhook can desync subscription state and a failed meter emission can
# drop usage-based charges.
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
