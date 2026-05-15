-- usage_events に決済区分を追加
--
-- paid_via:
--   'credit' : 全額クレジット残高から消費（Stripe には送られない）
--   'stripe' : 全額 Stripe usage-based meter で課金
--   'mixed'  : クレジット残高分 + Stripe meter 分が同一 event に発生
--
-- credit_amount_minor + stripe_amount_minor = cost_minor が成立する。
-- 端数なく按分するため、Stripe meter には cost を直接送る (tokens は参考値)。

ALTER TABLE usage_events
  ADD COLUMN IF NOT EXISTS paid_via            TEXT   NOT NULL DEFAULT 'stripe',
  ADD COLUMN IF NOT EXISTS credit_amount_minor BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS stripe_amount_minor BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_usage_events_paid_via
  ON usage_events(account_id, paid_via);
