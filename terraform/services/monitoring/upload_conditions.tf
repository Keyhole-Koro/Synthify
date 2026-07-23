# Upload rejection spike: the API emits UploadRejected for quota/size/type
# rejections. A spike is a misuse/attack signal or a regression in validation.
resource "newrelic_nrql_alert_condition" "upload_rejected" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.warning.id
  type                         = "static"
  name                         = "Upload rejection spike"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT count(*)
      FROM UploadRejected
    NRQL
  }

  warning {
    operator              = "above"
    threshold             = var.upload_rejected_count_threshold
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}
