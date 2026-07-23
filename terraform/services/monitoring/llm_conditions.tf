# LLM throttling spike: RetryingModel emits an LLMThrottled custom event for
# every retryable LLM failure. rate_limited events are a leading indicator of
# Vertex/Gemini quota starvation.
resource "newrelic_nrql_alert_condition" "llm_throttling" {
  account_id                   = var.new_relic_account_id
  policy_id                    = newrelic_alert_policy.warning.id
  type                         = "static"
  name                         = "LLM rate-limit/quota pressure"
  enabled                      = true
  violation_time_limit_seconds = 3600

  nrql {
    query = <<-NRQL
      SELECT count(*)
      FROM LLMThrottled
      WHERE reason = 'rate_limited'
    NRQL
  }

  warning {
    operator              = "above"
    threshold             = var.llm_throttle_count_threshold
    threshold_duration    = 300
    threshold_occurrences = "at_least_once"
  }

  fill_option        = "none"
  aggregation_window = 300
}
