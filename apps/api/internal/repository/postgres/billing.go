package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/postgres/sqlcgen"
)

// GetModelPricing returns the pricing row for the given model.
// Returns domain.ErrNotFound when the model is not registered.
func (s *Store) GetModelPricing(ctx context.Context, model string) (*domain.ModelPricing, error) {
	row, err := s.q().GetModelPricing(ctx, model)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	mult, _ := strconv.ParseFloat(row.DisplayMultiplier, 64)
	if mult == 0 {
		mult = 1.0
	}
	return &domain.ModelPricing{
		Model:                    row.Model,
		InputCostPerMTokenMinor:  row.InputCostPerMtokenMinor,
		OutputCostPerMTokenMinor: row.OutputCostPerMtokenMinor,
		Currency:                 row.Currency,
		DisplayMultiplier:        mult,
	}, nil
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
	usageDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", false, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	qtx := s.q().WithTx(tx)

	// 1. Insert raw event (idempotent via ON CONFLICT DO NOTHING).
	paidVia := string(ev.PaidVia)
	if paidVia == "" {
		paidVia = string(domain.PaidViaStripe)
	}
	if err := qtx.InsertUsageEvent(ctx, sqlcgen.InsertUsageEventParams{
		EventID:           ev.EventID,
		AccountID:         ev.AccountID,
		WorkspaceID:       ev.WorkspaceID,
		JobID:             ev.JobID,
		Model:             ev.Model,
		InputTokens:       ev.InputTokens,
		OutputTokens:      ev.OutputTokens,
		CostMinor:         ev.CostMinor,
		Currency:          ev.Currency,
		CreatedAt:         createdAt,
		PaidVia:           paidVia,
		CreditAmountMinor: ev.CreditAmountMinor,
		StripeAmountMinor: ev.StripeAmountMinor,
	}); err != nil {
		return "", false, err
	}

	// 2. Upsert daily rollup.
	if err := qtx.UpsertAccountUsageDaily(ctx, sqlcgen.UpsertAccountUsageDailyParams{
		AccountID:    ev.AccountID,
		UsageDate:    usageDate,
		Model:        ev.Model,
		InputTokens:  ev.InputTokens,
		OutputTokens: ev.OutputTokens,
		CostMinor:    ev.CostMinor,
		UpdatedAt:    now,
	}); err != nil {
		return "", false, err
	}

	// 3. Update account accumulator and check budget.
	var budgetLimitMinor int64
	var exceeded bool
	budgetRow, err := qtx.GetAccountBudgetForUsage(ctx, ev.AccountID)
	// If the UPDATE returns no rows (account missing) or budget columns not returned, ignore.
	if err != nil && err != sql.ErrNoRows {
		return "", false, err
	}

	if err == nil {
		budgetLimitMinor = budgetRow.BudgetLimitMinor
		exceeded = budgetRow.BudgetExceeded
	}
	if budgetLimitMinor > 0 {
		// Sum all costs for the account to check budget.
		totalMinor, sumErr := qtx.SumUsageCostByAccount(ctx, ev.AccountID)
		if sumErr == nil && totalMinor > budgetLimitMinor {
			exceeded = true
			_ = qtx.MarkAccountBudgetExceeded(ctx, ev.AccountID)
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
	rows, err := s.q().ListUsageByModel(ctx, sqlcgen.ListUsageByModelParams{
		AccountID:   accountID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	})
	if err != nil {
		return nil, "usd", err
	}

	var result []domain.ModelUsage
	currency := "usd"
	for _, row := range rows {
		if row.Currency != "" {
			currency = row.Currency
		}
		result = append(result, domain.ModelUsage{
			Model:        row.Model,
			InputTokens:  row.InputTokens,
			OutputTokens: row.OutputTokens,
			TotalCost:    formatMinorCost(row.CostMinor, row.Currency),
			EventCount:   row.EventCount,
		})
	}
	return result, currency, nil
}

// ListDailyUsage returns per-day total costs for the given period.
func (s *Store) ListDailyUsage(ctx context.Context, accountID, periodStart, periodEnd, currency string) ([]domain.DailyUsage, error) {
	rows, err := s.q().ListDailyUsage(ctx, sqlcgen.ListDailyUsageParams{
		AccountID:   accountID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	})
	if err != nil {
		return nil, err
	}

	var result []domain.DailyUsage
	for _, row := range rows {
		result = append(result, domain.DailyUsage{
			Date:      row.UsageDate,
			TotalCost: formatMinorCost(row.CostMinor, currency),
		})
	}
	return result, nil
}

// UpdateAccountBudgetLimit sets the monthly budget cap for the account.
// limitMinor == 0 means unlimited.
func (s *Store) UpdateAccountBudgetLimit(ctx context.Context, accountID string, limitMinor int64) error {
	return s.q().UpdateAccountBudgetLimit(ctx, sqlcgen.UpdateAccountBudgetLimitParams{
		AccountID:        accountID,
		BudgetLimitMinor: limitMinor,
		UpdatedAt:        nowTime(),
	})
}

// ListInvoices returns the most recent invoices for an account (Stripe-synced cache).
func (s *Store) ListInvoices(ctx context.Context, accountID string, limit int) (*domain.InvoiceList, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.q().ListInvoices(ctx, sqlcgen.ListInvoicesParams{
		AccountID: accountID,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, err
	}

	var invoices []*domain.Invoice
	for _, row := range rows {
		inv := &domain.Invoice{
			InvoiceID:        row.InvoiceID,
			Currency:         row.Currency,
			Status:           row.Status,
			HostedInvoiceURL: row.HostedInvoiceUrl,
			InvoicePDFURL:    row.InvoicePdfUrl,
			PeriodStart:      row.PeriodStart,
			PeriodEnd:        row.PeriodEnd,
			PaidAt:           row.PaidAt,
			CreatedAt:        row.CreatedAt,
		}
		inv.Amount = formatMinorCost(row.AmountMinor, inv.Currency)
		invoices = append(invoices, inv)
	}
	return &domain.InvoiceList{
		Invoices:          invoices,
		UpcomingAmount:    "0.00",
		UpcomingPeriodEnd: "",
	}, nil
}

// ListPaymentMethods returns stored payment methods for the account.
func (s *Store) ListPaymentMethods(ctx context.Context, accountID string) ([]*domain.PaymentMethod, error) {
	rows, err := s.q().ListPaymentMethods(ctx, accountID)
	if err != nil {
		return nil, err
	}

	var result []*domain.PaymentMethod
	for _, row := range rows {
		result = append(result, &domain.PaymentMethod{
			PaymentMethodID: row.PaymentMethodID,
			Brand:           row.Brand,
			Last4:           row.Last4,
			ExpMonth:        row.ExpMonth,
			ExpYear:         row.ExpYear,
			IsDefault:       row.IsDefault,
		})
	}
	return result, nil
}

// UpsertInvoice writes a Stripe-sourced invoice into the cache table. account_id is
// resolved from stripeCustomerID inside the query; an unlinked customer is a silent no-op.
func (s *Store) UpsertInvoice(ctx context.Context, stripeCustomerID string, inv *domain.ProviderInvoice) error {
	if inv == nil || stripeCustomerID == "" || inv.StripeInvoiceID == "" {
		return nil
	}
	currency := inv.Currency
	if currency == "" {
		currency = "usd"
	}
	return s.q().UpsertInvoice(ctx, sqlcgen.UpsertInvoiceParams{
		StripeInvoiceID:  inv.StripeInvoiceID,
		AmountMinor:      inv.AmountMinor,
		Currency:         currency,
		Status:           inv.Status,
		HostedInvoiceUrl: inv.HostedInvoiceURL,
		InvoicePdfUrl:    inv.InvoicePDFURL,
		PeriodStart:      nullTimeFromRFC3339(inv.PeriodStart),
		PeriodEnd:        nullTimeFromRFC3339(inv.PeriodEnd),
		PaidAt:           nullTimeFromRFC3339(inv.PaidAt),
		Ts:               timeFromRFC3339OrNow(inv.CreatedAt),
		StripeCustomerID: stripeCustomerID,
	})
}

// UpsertPaymentMethod writes a Stripe-sourced card into the cache table.
func (s *Store) UpsertPaymentMethod(ctx context.Context, stripeCustomerID string, pm *domain.ProviderPaymentMethod) error {
	if pm == nil || stripeCustomerID == "" || pm.StripePaymentMethodID == "" {
		return nil
	}
	return s.q().UpsertPaymentMethod(ctx, sqlcgen.UpsertPaymentMethodParams{
		StripePaymentMethodID: pm.StripePaymentMethodID,
		Brand:                 pm.Brand,
		Last4:                 pm.Last4,
		ExpMonth:              pm.ExpMonth,
		ExpYear:               pm.ExpYear,
		Ts:                    nowTime(),
		StripeCustomerID:      stripeCustomerID,
	})
}

// DeletePaymentMethod removes a card by its Stripe id (payment_method.detached).
func (s *Store) DeletePaymentMethod(ctx context.Context, stripePaymentMethodID string) error {
	if stripePaymentMethodID == "" {
		return nil
	}
	return s.q().DeletePaymentMethod(ctx, stripePaymentMethodID)
}

// SetDefaultPaymentMethod flips is_default across the account's cards so that only the
// one matching defaultPaymentMethodID is true (empty id clears all defaults).
func (s *Store) SetDefaultPaymentMethod(ctx context.Context, stripeCustomerID, defaultPaymentMethodID string) error {
	if stripeCustomerID == "" {
		return nil
	}
	return s.q().SetDefaultPaymentMethodByCustomer(ctx, sqlcgen.SetDefaultPaymentMethodByCustomerParams{
		DefaultPaymentMethodID: defaultPaymentMethodID,
		Ts:                     nowTime(),
		StripeCustomerID:       stripeCustomerID,
	})
}

// nullTimeFromRFC3339 parses an RFC3339 string into a sql.NullTime; "" / parse error → NULL.
func nullTimeFromRFC3339(value string) sql.NullTime {
	if value == "" {
		return sql.NullTime{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// timeFromRFC3339OrNow parses an RFC3339 string, falling back to now() on empty/invalid.
func timeFromRFC3339OrNow(value string) time.Time {
	if value != "" {
		if t, err := time.Parse(time.RFC3339, value); err == nil {
			return t
		}
	}
	return nowTime()
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

// GrantCredit inserts a credit row. Consumed credits are inserted with a negative amount.
func (s *Store) GrantCredit(ctx context.Context, grant *domain.CreditGrant) error {
	grantedAt, err := time.Parse(time.RFC3339, grant.GrantedAt)
	if err != nil {
		grantedAt = nowTime()
	}
	return s.q().GrantCredit(ctx, sqlcgen.GrantCreditParams{
		CreditID:   grant.CreditID,
		AccountID:  grant.AccountID,
		CreditType: string(grant.CreditType),
		AmountMinor: grant.AmountMinor,
		Currency:   grant.Currency,
		Note:       grant.Note,
		GrantedBy:  grant.GrantedBy,
		GrantedAt:  grantedAt,
	})
}

// GetCreditBalance returns the net credit balance (granted - consumed) in minor units.
func (s *Store) GetCreditBalance(ctx context.Context, accountID string) (int64, error) {
	return s.q().GetCreditBalance(ctx, accountID)
}

// ListCreditGrants returns recent credit transactions for an account.
func (s *Store) ListCreditGrants(ctx context.Context, accountID string, limit int) ([]*domain.CreditGrant, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.q().ListCreditGrants(ctx, sqlcgen.ListCreditGrantsParams{
		AccountID: accountID,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CreditGrant, 0, len(rows))
	for _, row := range rows {
		result = append(result, &domain.CreditGrant{
			CreditID:    row.CreditID,
			AccountID:   accountID,
			CreditType:  domain.CreditType(row.CreditType),
			AmountMinor: row.AmountMinor,
			Currency:    row.Currency,
			Note:        row.Note,
			GrantedBy:   row.GrantedBy,
			GrantedAt:   row.GrantedAt.UTC().Format(time.RFC3339),
		})
	}
	return result, nil
}
