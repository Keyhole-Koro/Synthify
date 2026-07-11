# Discord accepts Slack-formatted incoming webhooks by appending /slack to
# the webhook URL, so we reuse New Relic's built-in Slack destination type
# instead of standing up a payload-translation relay.
resource "newrelic_notification_destination" "discord" {
  account_id = var.new_relic_account_id
  name       = "discord-alerts-${var.environment}"
  type       = "WEBHOOK"

  property {
    key   = "url"
    value = "${var.discord_alert_webhook_url}/slack"
  }
}

resource "newrelic_notification_channel" "discord" {
  account_id     = var.new_relic_account_id
  name           = "discord-alerts-channel-${var.environment}"
  type           = "WEBHOOK"
  destination_id = newrelic_notification_destination.discord.id
  product        = "IINT"

  property {
    key = "payload"
    value = jsonencode({
      text = "{{issueTitle}}\n{{issueUrl}}"
    })
  }
}

resource "newrelic_workflow" "critical" {
  account_id            = var.new_relic_account_id
  name                  = "synthify-critical-${var.environment}"
  muting_rules_handling = "NOTIFY_ALL_ISSUES"

  issues_filter {
    name = "critical-policy-filter"
    type = "FILTER"

    predicate {
      attribute = "labels.policyIds"
      operator  = "EXACTLY_MATCHES"
      values    = [newrelic_alert_policy.critical.id]
    }
  }

  destination {
    channel_id = newrelic_notification_channel.discord.id
  }
}

resource "newrelic_workflow" "warning" {
  account_id            = var.new_relic_account_id
  name                  = "synthify-warning-${var.environment}"
  muting_rules_handling = "NOTIFY_ALL_ISSUES"

  issues_filter {
    name = "warning-policy-filter"
    type = "FILTER"

    predicate {
      attribute = "labels.policyIds"
      operator  = "EXACTLY_MATCHES"
      values    = [newrelic_alert_policy.warning.id]
    }
  }

  destination {
    channel_id = newrelic_notification_channel.discord.id
  }
}
