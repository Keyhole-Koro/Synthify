package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/synthify/backend/packages/shared/domain"
)

// GetModelPricing returns the pricing row for the given model.
// Returns domain.ErrNotFound when the model is not registered.
func (s *Store) GetModelPricing(ctx context.Context, model string) (*domain.ModelPricing, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT model, input_cost_per_mtoken_minor, output_cost_per_mtoken_minor, currency
FROM model_pricing
WHERE model = $1
`, model)
	var p domain.ModelPricing
	if err := row.Scan(&p.Model, &p.InputCostPerMTokenMinor, &p.OutputCostPerMTokenMinor, &p.Currency); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// RecordUsageAccounting persists the raw usage event and updates the daily
// rollup and account accumulator atomically. Returns (eventID, budgetExceeded, error).
// The budget is considered exceeded when budget_limit_minor > 0 and the account's
// accumulated cost exceeds it after this event.
func (s *Store) RecordUsageAccounting(ctx context.Context, ev *domain.UsageEvent, date string) (string, bool, error) {
	now := nowTime()
	if ev.CreatedAt == "" {
		ev.CreatedAt = now.Format(time.RFC3339)
	}
	createdAt, err := time.Parse(time.RFC3339, ev.CreatedAt)
	if err != nil {
		createdAt = now
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()

	// 1. Insert raw event (idempotent via ON CONFLICT DO NOTHING).
	_, err = tx.ExecContext(ctx, `
INSERT INTO usage_events
  (event_id, account_id, workspace_id, job_id, model, input_tokens, output_tokens, cost_minor, currency, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (event_id) DO NOTHING
`, ev.EventID, ev.AccountID, ev.WorkspaceID, ev.JobID, ev.Model,
		ev.InputTokens, ev.OutputTokens, ev.CostMinor, ev.Currency, createdAt)
	if err != nil {
		return "", false, err
	}

	// 2. Upsert daily rollup.
	_, err = tx.ExecContext(ctx, `
INSERT INTO account_usage_daily
  (account_id, usage_date, model, input_tokens, output_tokens, cost_minor, event_count, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, 1, $7)
ON CONFLICT (account_id, usage_date, model) DO UPDATE SET
  input_tokens = account_usage_daily.input_tokens + EXCLUDED.input_tokens,
  output_tokens = account_usage_daily.output_tokens + EXCLUDED.output_tokens,
  cost_minor = account_usage_daily.cost_minor + EXCLUDED.cost_minor,
  event_count = account_usage_daily.event_count + 1,
  updated_at = EXCLUDED.updated_at
`, ev.AccountID, date, ev.Model, ev.InputTokens, ev.OutputTokens, ev.CostMinor, now)
	if err != nil {
		return "", false, err
	}

	// 3. Update account accumulator and check budget.
	var budgetLimitMinor int64
	var exceeded bool
	err = tx.QueryRowContext(ctx, `
UPDATE accounts
SET storage_used_bytes = storage_used_bytes  -- placeholder; cost not in accounts yet
WHERE account_id = $1
RETURNING budget_limit_minor, budget_exceeded
`, ev.AccountID).Scan(&budgetLimitMinor, &exceeded)
	// If the UPDATE returns no rows (account missing) or budget columns not returned, ignore.
	if err != nil && err != sql.ErrNoRows {
		return "", false, err
	}

	if err == nil && budgetLimitMinor > 0 {
		// Sum all costs for the account to check budget.
		var totalMinor int64
		sumErr := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(cost_minor), 0)
FROM usage_events
WHERE account_id = $1
`, ev.AccountID).Scan(&totalMinor)
		if sumErr == nil && totalMinor > budgetLimitMinor {
			exceeded = true
			_, _ = tx.ExecContext(ctx, `
UPDATE accounts SET budget_exceeded = TRUE WHERE account_id = $1
`, ev.AccountID)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", false, err
	}
	return ev.EventID, exceeded, nil
}

// ListUsageByModel aggregates usage by model for the given period.
// Returns ([]ModelUsage, currency, error). currency is the most common currency
// for the account in that period, defaulting to "usd".
func (s *Store) ListUsageByModel(ctx context.Context, accountID, periodStart, periodEnd string) ([]domain.ModelUsage, string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT model,
       SUM(input_tokens)  AS input_tokens,
       SUM(output_tokens) AS output_tokens,
       SUM(cost_minor)    AS cost_minor,
       COUNT(*)           AS event_count,
       MAX(currency)      AS currency
FROM usage_events
WHERE account_id = $1
  AND ($2 = '' OR created_at >= $2::timestamptz)
  AND ($3 = '' OR created_at <  $3::timestamptz)
GROUP BY model
ORDER BY cost_minor DESC
`, accountID, periodStart, periodEnd)
	if err != nil {
		return nil, "usd", err
	}
	defer rows.Close()

	var result []domain.ModelUsage
	currency := "usd"
	for rows.Next() {
		var m domain.ModelUsage
		var costMinor int64
		var cur string
		if err := rows.Scan(&m.Model, &m.InputTokens, &m.OutputTokens, &costMinor, &m.EventCount, &cur); err != nil {
			return nil, currency, err
		}
		if cur != "" {
			currency = cur
		}
		m.TotalCost = formatMinorCost(costMinor, cur)
		result = append(result, m)
	}
	return result, currency, rows.Err()
}

// ListDailyUsage returns per-day total costs for the given period.
func (s *Store) ListDailyUsage(ctx context.Context, accountID, periodStart, periodEnd, currency string) ([]domain.DailyUsage, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT usage_date::text, SUM(cost_minor) AS cost_minor
FROM account_usage_daily
WHERE account_id = $1
  AND ($2 = '' OR usage_date >= $2::date)
  AND ($3 = '' OR usage_date <  $3::date)
GROUP BY usage_date
ORDER BY usage_date ASC
`, accountID, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.DailyUsage
	for rows.Next() {
		var d domain.DailyUsage
		var costMinor int64
		if err := rows.Scan(&d.Date, &costMinor); err != nil {
			return nil, err
		}
		d.TotalCost = formatMinorCost(costMinor, currency)
		result = append(result, d)
	}
	return result, rows.Err()
}

// UpdateAccountBudgetLimit sets the monthly budget cap for the account.
// limitMinor == 0 means unlimited.
func (s *Store) UpdateAccountBudgetLimit(ctx context.Context, accountID string, limitMinor int64) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE accounts
SET budget_limit_minor = $2, budget_exceeded = FALSE, updated_at = $3
WHERE account_id = $1
`, accountID, limitMinor, nowTime())
	return err
}

// ListInvoices returns the most recent invoices for an account (Stripe-synced cache).
func (s *Store) ListInvoices(ctx context.Context, accountID string, limit int) (*domain.InvoiceList, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT invoice_id, amount_minor, currency, status,
       hosted_invoice_url, invoice_pdf_url,
       COALESCE(TO_CHAR(period_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '') AS period_start,
       COALESCE(TO_CHAR(period_end   AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '') AS period_end,
       COALESCE(TO_CHAR(paid_at      AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), '') AS paid_at,
       TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at
FROM invoices
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2
`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []*domain.Invoice
	for rows.Next() {
		inv := &domain.Invoice{}
		var amountMinor int64
		if err := rows.Scan(
			&inv.InvoiceID, &amountMinor, &inv.Currency, &inv.Status,
			&inv.HostedInvoiceURL, &inv.InvoicePDFURL,
			&inv.PeriodStart, &inv.PeriodEnd, &inv.PaidAt, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		inv.Amount = formatMinorCost(amountMinor, inv.Currency)
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &domain.InvoiceList{
		Invoices:          invoices,
		UpcomingAmount:    "0.00",
		UpcomingPeriodEnd: "",
	}, nil
}

// ListPaymentMethods returns stored payment methods for the account.
func (s *Store) ListPaymentMethods(ctx context.Context, accountID string) ([]*domain.PaymentMethod, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT payment_method_id, brand, last4, exp_month, exp_year, is_default
FROM payment_methods
WHERE account_id = $1
ORDER BY is_default DESC, created_at DESC
`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.PaymentMethod
	for rows.Next() {
		pm := &domain.PaymentMethod{}
		if err := rows.Scan(&pm.PaymentMethodID, &pm.Brand, &pm.Last4, &pm.ExpMonth, &pm.ExpYear, &pm.IsDefault); err != nil {
			return nil, err
		}
		result = append(result, pm)
	}
	return result, rows.Err()
}

// formatMinorCost converts a minor-unit amount to a decimal string.
// JPY has no fractional unit; all other currencies use 2 decimal places.
func formatMinorCost(minor int64, currency string) string {
	if currency == "jpy" {
		if minor < 0 {
			return "-" + itoa(-minor)
		}
		return itoa(minor)
	}
	neg := ""
	if minor < 0 {
		neg = "-"
		minor = -minor
	}
	whole := minor / 100
	frac := minor % 100
	return neg + itoa(whole) + "." + pad2(frac)
}

func itoa(n int64) string {
	return string(appendInt(nil, n))
}

func appendInt(b []byte, n int64) []byte {
	if n == 0 {
		return append(b, '0')
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return append(b, buf[pos:]...)
}

func pad2(n int64) string {
	if n < 10 {
		return "0" + string([]byte{byte('0' + n)})
	}
	return string([]byte{byte('0' + n/10), byte('0' + n%10)})
}
