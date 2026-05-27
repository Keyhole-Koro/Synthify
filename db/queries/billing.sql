-- name: GetModelPricing :one
SELECT model, input_cost_per_mtoken_minor, output_cost_per_mtoken_minor, currency, display_multiplier
FROM model_pricing
WHERE model = $1;

-- name: GrantCredit :exec
INSERT INTO account_credits
  (credit_id, account_id, credit_type, amount_minor, currency, note, granted_by, granted_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetCreditBalance :one
SELECT COALESCE(SUM(amount_minor), 0)::bigint AS credit_balance_minor
FROM account_credits
WHERE account_id = $1;

-- name: ListCreditGrants :many
SELECT credit_id, credit_type, amount_minor, currency, note, granted_by, granted_at
FROM account_credits
WHERE account_id = $1
ORDER BY granted_at DESC
LIMIT $2;

-- name: InsertUsageEvent :exec
INSERT INTO usage_events
  (event_id, account_id, workspace_id, job_id, model, input_tokens, output_tokens, cost_minor, currency, created_at,
   paid_via, credit_amount_minor, stripe_amount_minor)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (event_id) DO NOTHING;

-- name: UpsertAccountUsageDaily :exec
INSERT INTO account_usage_daily
  (account_id, usage_date, model, input_tokens, output_tokens, cost_minor, event_count, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
ON CONFLICT (account_id, usage_date, model) DO UPDATE SET
  input_tokens = account_usage_daily.input_tokens + EXCLUDED.input_tokens,
  output_tokens = account_usage_daily.output_tokens + EXCLUDED.output_tokens,
  cost_minor = account_usage_daily.cost_minor + EXCLUDED.cost_minor,
  event_count = account_usage_daily.event_count + 1,
  updated_at = EXCLUDED.updated_at;

-- name: GetAccountBudgetForUsage :one
UPDATE accounts
SET storage_used_bytes = storage_used_bytes
WHERE account_id = $1
RETURNING budget_limit_minor, budget_exceeded;

-- name: SumUsageCostByAccount :one
SELECT COALESCE(SUM(cost_minor), 0)::bigint
FROM usage_events
WHERE account_id = $1;

-- name: MarkAccountBudgetExceeded :exec
UPDATE accounts SET budget_exceeded = TRUE WHERE account_id = $1;

-- name: DeductCredit :exec
-- クレジット残高から消費分を差し引く（負の amount_minor を挿入することで残高を減らす）
INSERT INTO account_credits
  (credit_id, account_id, credit_type, amount_minor, currency, note, granted_by, granted_at)
VALUES ($1, $2, 'consumed', $3, $4, $5, 'system', $6);

-- name: ListUsageByModel :many
SELECT model,
       SUM(input_tokens)::bigint  AS input_tokens,
       SUM(output_tokens)::bigint AS output_tokens,
       SUM(cost_minor)::bigint    AS cost_minor,
       COUNT(*)::bigint           AS event_count,
       MAX(currency)::text        AS currency
FROM usage_events
WHERE account_id = sqlc.arg(account_id)
  AND (sqlc.arg(period_start)::text = '' OR created_at >= sqlc.arg(period_start)::timestamptz)
  AND (sqlc.arg(period_end)::text = '' OR created_at <  sqlc.arg(period_end)::timestamptz)
GROUP BY model
ORDER BY cost_minor DESC;

-- name: ListDailyUsage :many
SELECT usage_date::text, SUM(cost_minor)::bigint AS cost_minor
FROM account_usage_daily
WHERE account_id = sqlc.arg(account_id)
  AND (sqlc.arg(period_start)::text = '' OR usage_date >= sqlc.arg(period_start)::date)
  AND (sqlc.arg(period_end)::text = '' OR usage_date <  sqlc.arg(period_end)::date)
GROUP BY usage_date
ORDER BY usage_date ASC;

-- name: UpdateAccountBudgetLimit :exec
UPDATE accounts
SET budget_limit_minor = $2, budget_exceeded = FALSE, updated_at = $3
WHERE account_id = $1;

-- name: ListInvoices :many
SELECT invoice_id, amount_minor, currency, status,
       hosted_invoice_url, invoice_pdf_url,
       COALESCE(TO_CHAR(period_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')::text AS period_start,
       COALESCE(TO_CHAR(period_end   AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')::text AS period_end,
       COALESCE(TO_CHAR(paid_at      AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '')::text AS paid_at,
       TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
FROM invoices
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListPaymentMethods :many
SELECT payment_method_id, brand, last4, exp_month, exp_year, is_default
FROM payment_methods
WHERE account_id = $1
ORDER BY is_default DESC, created_at DESC;

-- name: ApplyBillingPlan :execrows
-- plan に応じた quota / 上限を一括で反映する。
-- stripe_customer_id は空文字なら既存値を保持 (CASE)。
UPDATE accounts
SET plan = @plan::text,
    storage_quota_bytes = @storage_quota_bytes::bigint,
    max_file_size_bytes = @max_file_size_bytes::bigint,
    max_uploads_per_5h = @max_uploads_per_5h::int,
    max_uploads_per_1week = @max_uploads_per_1week::int,
    stripe_customer_id = CASE WHEN @stripe_customer_id::text = '' THEN stripe_customer_id ELSE @stripe_customer_id::text END,
    stripe_subscription_id = @stripe_subscription_id::text,
    billing_status = CASE WHEN @plan::text = 'free' THEN 'free' ELSE 'active' END,
    billing_updated_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE account_id = @account_id;

-- name: ApplyBillingPlanByStripeCustomerID :execrows
UPDATE accounts
SET plan = @plan::text,
    storage_quota_bytes = @storage_quota_bytes::bigint,
    max_file_size_bytes = @max_file_size_bytes::bigint,
    max_uploads_per_5h = @max_uploads_per_5h::int,
    max_uploads_per_1week = @max_uploads_per_1week::int,
    stripe_subscription_id = @stripe_subscription_id::text,
    billing_status = CASE WHEN @plan::text = 'free' THEN 'free' ELSE 'active' END,
    billing_updated_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE stripe_customer_id = @stripe_customer_id;

-- name: RecordBillingWebhookEvent :execrows
-- 重複 (provider, event_id) を ON CONFLICT で吸収。
-- 戻り値の rows_affected で「新規受信か既受信か」を判定する。
INSERT INTO billing_events (
  provider, event_id, event_type, received_at, processing_status,
  account_id, stripe_customer_id, stripe_subscription_id
) VALUES ($1, $2, $3, $4, 'received', $5, $6, $7)
ON CONFLICT (provider, event_id) DO NOTHING;

-- name: MarkBillingWebhookEventProcessed :exec
UPDATE billing_events
SET processing_status = $3, error_message = $4, processed_at = $5
WHERE provider = $1 AND event_id = $2;

-- name: ApplyBillingEventByAccount :execrows
-- ApplyBillingEvent の account_id 経路。
-- 空文字パラメータは既存値を保持する CASE で表現する。
-- plan が '' なら plan / quota 関連は据え置き。
-- stripe_subscription_id は plan!='free' の場合のみ「空なら据え置き」。
UPDATE accounts
SET plan = CASE WHEN @plan::text = '' THEN plan ELSE @plan::text END,
    storage_quota_bytes = CASE WHEN @plan::text = '' THEN storage_quota_bytes ELSE @storage_quota_bytes::bigint END,
    max_file_size_bytes = CASE WHEN @plan::text = '' THEN max_file_size_bytes ELSE @max_file_size_bytes::bigint END,
    max_uploads_per_5h = CASE WHEN @plan::text = '' THEN max_uploads_per_5h ELSE @max_uploads_per_5h::int END,
    max_uploads_per_1week = CASE WHEN @plan::text = '' THEN max_uploads_per_1week ELSE @max_uploads_per_1week::int END,
    stripe_customer_id = CASE WHEN @stripe_customer_id::text = '' THEN stripe_customer_id ELSE @stripe_customer_id::text END,
    stripe_subscription_id = CASE WHEN @stripe_subscription_id::text = '' AND @plan::text <> 'free' THEN stripe_subscription_id ELSE @stripe_subscription_id::text END,
    billing_status = @billing_status,
    stripe_price_id = @stripe_price_id,
    billing_currency = @billing_currency,
    billing_amount_minor = @billing_amount_minor,
    billing_interval = @billing_interval,
    current_period_end = @current_period_end,
    cancel_at_period_end = @cancel_at_period_end,
    billing_updated_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE account_id = @account_id;

-- name: ApplyBillingEventByStripeCustomer :execrows
-- ApplyBillingEvent の stripe_customer_id 経路。account_id が未設定の webhook で使う。
UPDATE accounts
SET plan = CASE WHEN @plan::text = '' THEN plan ELSE @plan::text END,
    storage_quota_bytes = CASE WHEN @plan::text = '' THEN storage_quota_bytes ELSE @storage_quota_bytes::bigint END,
    max_file_size_bytes = CASE WHEN @plan::text = '' THEN max_file_size_bytes ELSE @max_file_size_bytes::bigint END,
    max_uploads_per_5h = CASE WHEN @plan::text = '' THEN max_uploads_per_5h ELSE @max_uploads_per_5h::int END,
    max_uploads_per_1week = CASE WHEN @plan::text = '' THEN max_uploads_per_1week ELSE @max_uploads_per_1week::int END,
    stripe_customer_id = CASE WHEN @stripe_customer_id_passthru::text = '' THEN stripe_customer_id ELSE @stripe_customer_id_passthru::text END,
    stripe_subscription_id = CASE WHEN @stripe_subscription_id::text = '' AND @plan::text <> 'free' THEN stripe_subscription_id ELSE @stripe_subscription_id::text END,
    billing_status = @billing_status,
    stripe_price_id = @stripe_price_id,
    billing_currency = @billing_currency,
    billing_amount_minor = @billing_amount_minor,
    billing_interval = @billing_interval,
    current_period_end = @current_period_end,
    cancel_at_period_end = @cancel_at_period_end,
    billing_updated_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE stripe_customer_id = @stripe_customer_id_match;
