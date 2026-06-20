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

-- name: UpsertInvoice :exec
-- account_id は stripe_customer_id から解決する。一致する account が無ければ
-- SELECT が 0 行となり何も挿入されない（未連携顧客の invoice は黙って捨てる）。
-- invoice_id には stripe_invoice_id をそのまま採用し、ON CONFLICT で冪等更新する。
INSERT INTO invoices (
  invoice_id, account_id, stripe_invoice_id, amount_minor, currency, status,
  hosted_invoice_url, invoice_pdf_url, period_start, period_end, paid_at,
  created_at, updated_at
)
SELECT
  sqlc.arg(stripe_invoice_id), a.account_id, sqlc.arg(stripe_invoice_id),
  sqlc.arg(amount_minor), sqlc.arg(currency), sqlc.arg(status),
  sqlc.arg(hosted_invoice_url), sqlc.arg(invoice_pdf_url),
  sqlc.narg(period_start)::timestamptz,
  sqlc.narg(period_end)::timestamptz,
  sqlc.narg(paid_at)::timestamptz,
  sqlc.arg(ts)::timestamptz, sqlc.arg(ts)::timestamptz
FROM accounts a
WHERE a.stripe_customer_id = sqlc.arg(stripe_customer_id)
LIMIT 1
ON CONFLICT (invoice_id) DO UPDATE SET
  amount_minor       = EXCLUDED.amount_minor,
  status             = EXCLUDED.status,
  hosted_invoice_url = EXCLUDED.hosted_invoice_url,
  invoice_pdf_url    = EXCLUDED.invoice_pdf_url,
  period_start       = EXCLUDED.period_start,
  period_end         = EXCLUDED.period_end,
  paid_at            = EXCLUDED.paid_at,
  updated_at         = EXCLUDED.updated_at;

-- name: UpsertPaymentMethod :exec
-- account_id は stripe_customer_id から解決。is_default は SetDefaultPaymentMethodByCustomer
-- が customer.updated イベントで別途管理するため、ここでは insert 時 FALSE 固定・更新時は触らない。
INSERT INTO payment_methods (
  payment_method_id, account_id, stripe_payment_method_id,
  brand, last4, exp_month, exp_year, is_default, created_at, updated_at
)
SELECT
  sqlc.arg(stripe_payment_method_id), a.account_id, sqlc.arg(stripe_payment_method_id),
  sqlc.arg(brand), sqlc.arg(last4), sqlc.arg(exp_month), sqlc.arg(exp_year),
  FALSE, sqlc.arg(ts)::timestamptz, sqlc.arg(ts)::timestamptz
FROM accounts a
WHERE a.stripe_customer_id = sqlc.arg(stripe_customer_id)
LIMIT 1
ON CONFLICT (payment_method_id) DO UPDATE SET
  brand      = EXCLUDED.brand,
  last4      = EXCLUDED.last4,
  exp_month  = EXCLUDED.exp_month,
  exp_year   = EXCLUDED.exp_year,
  updated_at = EXCLUDED.updated_at;

-- name: DeletePaymentMethod :exec
DELETE FROM payment_methods
WHERE stripe_payment_method_id = sqlc.arg(stripe_payment_method_id);

-- name: SetDefaultPaymentMethodByCustomer :exec
-- 該当 account の全 PM のうち default_payment_method_id に一致する 1 件だけ is_default=TRUE、
-- 残りを FALSE にする。default_payment_method_id が空なら全件 FALSE。
UPDATE payment_methods pm
SET is_default = (pm.stripe_payment_method_id = sqlc.arg(default_payment_method_id)),
    updated_at = sqlc.arg(ts)::timestamptz
FROM accounts a
WHERE pm.account_id = a.account_id
  AND a.stripe_customer_id = sqlc.arg(stripe_customer_id);
