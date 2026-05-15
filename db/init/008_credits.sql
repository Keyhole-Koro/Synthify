-- Credits (無料クレジット・プリペイド残高)
--
--   account_credits : 付与・購入単位でクレジットを記録し残高を計算する
--
-- credit_type:
--   "free"     : サインアップ時の無料付与、admin による手動付与
--   "purchased": Stripe Payment Intent で購入したクレジット
--
-- amount_minor は USD cents。消費は usage_events.cost_minor を合計して
-- 残高 = SUM(granted) - SUM(consumed) で算出する（行単位で消費を追わない）。
-- 残高チェックは RecordUsageAccounting の中で行い、負になったらジョブ停止フラグを立てる。

CREATE TABLE IF NOT EXISTS account_credits (
  credit_id       TEXT PRIMARY KEY,
  account_id      TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
  credit_type     TEXT NOT NULL,          -- 'free' | 'purchased'
  amount_minor    BIGINT NOT NULL,        -- 付与額 (USD cents)
  currency        TEXT NOT NULL DEFAULT 'usd',
  note            TEXT NOT NULL DEFAULT '',
  granted_by      TEXT NOT NULL DEFAULT '', -- admin user_id or 'system'
  granted_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_account_credits_account
  ON account_credits(account_id, granted_at DESC);

-- model_pricing に表示倍率を追加
ALTER TABLE model_pricing
  ADD COLUMN IF NOT EXISTS display_multiplier NUMERIC(6,2) NOT NULL DEFAULT 1.0;
