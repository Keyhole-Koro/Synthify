-- name: GetModelPricing :one
SELECT model, input_cost_per_mtoken_minor, output_cost_per_mtoken_minor, currency
FROM model_pricing
WHERE model = $1;

-- name: InsertUsageEvent :exec
INSERT INTO usage_events
  (event_id, account_id, workspace_id, job_id, model, input_tokens, output_tokens, cost_minor, currency, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
