-- Billing (Stripe webhook & subscription state)
--   billing_events : Stripe webhook の冪等記録

CREATE TABLE IF NOT EXISTS billing_events (
  provider TEXT NOT NULL,
  event_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  received_at TIMESTAMPTZ NOT NULL,
  processed_at TIMESTAMPTZ,
  processing_status TEXT NOT NULL,
  error_message TEXT NOT NULL DEFAULT '',
  account_id TEXT NOT NULL DEFAULT '',
  stripe_customer_id TEXT NOT NULL DEFAULT '',
  stripe_subscription_id TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (provider, event_id)
);

CREATE INDEX IF NOT EXISTS idx_billing_events_account_id ON billing_events(account_id);
