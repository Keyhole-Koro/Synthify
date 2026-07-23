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
