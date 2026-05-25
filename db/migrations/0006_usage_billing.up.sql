-- Usage-Based Billing (Phase 1-3)
-- 詳細: docs/improvements/usage-based-billing.md
-- 金額は *_minor (USD なら cents)。API レスポンスは decimal string で返す。
--
--   usage_events         : LLM コール単位の使用量記録
--   account_usage_daily  : 日次ロールアップ
--   model_pricing        : モデル別単価
--   invoices             : Stripe 請求書キャッシュ
--   payment_methods      : Stripe 支払い方法キャッシュ

CREATE TABLE IF NOT EXISTS usage_events (
  event_id            TEXT PRIMARY KEY,
  account_id          TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  workspace_id        TEXT NOT NULL DEFAULT '',
  job_id              TEXT NOT NULL DEFAULT '',
  model               TEXT NOT NULL,
  input_tokens        BIGINT NOT NULL DEFAULT 0,
  output_tokens       BIGINT NOT NULL DEFAULT 0,
  cost_minor          BIGINT NOT NULL DEFAULT 0,
  currency            TEXT NOT NULL DEFAULT 'usd',
  created_at          TIMESTAMPTZ NOT NULL,
  paid_via            TEXT NOT NULL DEFAULT 'stripe',
  credit_amount_minor BIGINT NOT NULL DEFAULT 0,
  stripe_amount_minor BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_usage_events_account_created
  ON usage_events(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_events_job
  ON usage_events(job_id) WHERE job_id <> '';

CREATE TABLE IF NOT EXISTS account_usage_daily (
  account_id      TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  usage_date      DATE NOT NULL,
  model           TEXT NOT NULL,
  input_tokens    BIGINT NOT NULL DEFAULT 0,
  output_tokens   BIGINT NOT NULL DEFAULT 0,
  cost_minor      BIGINT NOT NULL DEFAULT 0,
  event_count     INTEGER NOT NULL DEFAULT 0,
  updated_at      TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (account_id, usage_date, model)
);

CREATE INDEX IF NOT EXISTS idx_account_usage_daily_account_date
  ON account_usage_daily(account_id, usage_date DESC);

CREATE TABLE IF NOT EXISTS model_pricing (
  model                        TEXT PRIMARY KEY,
  input_cost_per_mtoken_minor  BIGINT NOT NULL,
  output_cost_per_mtoken_minor BIGINT NOT NULL,
  currency                     TEXT NOT NULL DEFAULT 'usd',
  effective_from               TIMESTAMPTZ NOT NULL,
  notes                        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS invoices (
  invoice_id              TEXT PRIMARY KEY,
  account_id              TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  stripe_invoice_id       TEXT NOT NULL DEFAULT '',
  amount_minor            BIGINT NOT NULL DEFAULT 0,
  currency                TEXT NOT NULL DEFAULT 'usd',
  status                  TEXT NOT NULL,
  hosted_invoice_url      TEXT NOT NULL DEFAULT '',
  invoice_pdf_url         TEXT NOT NULL DEFAULT '',
  period_start            TIMESTAMPTZ,
  period_end              TIMESTAMPTZ,
  finalized_at            TIMESTAMPTZ,
  paid_at                 TIMESTAMPTZ,
  created_at              TIMESTAMPTZ NOT NULL,
  updated_at              TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_invoices_account_created
  ON invoices(account_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_invoices_stripe
  ON invoices(stripe_invoice_id) WHERE stripe_invoice_id <> '';

CREATE TABLE IF NOT EXISTS payment_methods (
  payment_method_id        TEXT PRIMARY KEY,
  account_id               TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  stripe_payment_method_id TEXT NOT NULL,
  brand                    TEXT NOT NULL DEFAULT '',
  last4                    TEXT NOT NULL DEFAULT '',
  exp_month                INTEGER NOT NULL DEFAULT 0,
  exp_year                 INTEGER NOT NULL DEFAULT 0,
  is_default               BOOL NOT NULL DEFAULT FALSE,
  created_at               TIMESTAMPTZ NOT NULL,
  updated_at               TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_payment_methods_account
  ON payment_methods(account_id, is_default DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_methods_stripe
  ON payment_methods(stripe_payment_method_id);
