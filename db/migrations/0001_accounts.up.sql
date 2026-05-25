-- Accounts & members
--   accounts        : 課金主体。Stripe 連携・ストレージ quota・予算を保持
--   account_users   : account にぶら下がるユーザー (role 付き)

CREATE TABLE IF NOT EXISTS accounts (
  account_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  plan TEXT NOT NULL,
  storage_quota_bytes BIGINT NOT NULL DEFAULT 0,
  storage_used_bytes BIGINT NOT NULL DEFAULT 0,
  max_file_size_bytes BIGINT NOT NULL DEFAULT 0,
  max_uploads_per_5h INTEGER NOT NULL DEFAULT 0,
  max_uploads_per_1week INTEGER NOT NULL DEFAULT 0,
  stripe_customer_id TEXT NOT NULL DEFAULT '',
  stripe_subscription_id TEXT NOT NULL DEFAULT '',
  billing_status TEXT NOT NULL DEFAULT 'free',
  stripe_price_id TEXT NOT NULL DEFAULT '',
  billing_currency TEXT NOT NULL DEFAULT '',
  billing_amount_minor BIGINT NOT NULL DEFAULT 0,
  billing_interval TEXT NOT NULL DEFAULT '',
  current_period_end TIMESTAMPTZ,
  cancel_at_period_end BOOL NOT NULL DEFAULT FALSE,
  billing_updated_at TIMESTAMPTZ,
  -- Usage-Based Billing (Phase 2): 月次予算と進捗
  budget_limit_minor BIGINT NOT NULL DEFAULT 0,           -- 0 = 無制限
  current_period_usage_minor BIGINT NOT NULL DEFAULT 0,
  current_period_started_at TIMESTAMPTZ,
  budget_exceeded BOOL NOT NULL DEFAULT FALSE,
  budget_alert_80_sent_at TIMESTAMPTZ,
  budget_alert_100_sent_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS account_users (
  account_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  user_id TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'member',
  joined_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (account_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_account_users_user_id ON account_users(user_id);
