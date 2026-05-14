package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/synthify/backend/packages/shared/domain"
	"github.com/synthify/backend/packages/shared/repository/postgres/sqlcgen"
)

// defaultFreePlan defines the default plan limits for newly created accounts.
var defaultFreePlan = struct {
	StorageQuotaBytes int64
	MaxFileSizeBytes  int64
	MaxUploadsPer5h   int64
	MaxUploadsPerWeek int64
}{
	StorageQuotaBytes: 5 * 1 << 30, // 5GB
	MaxFileSizeBytes:  100 << 20,   // 100MB
	MaxUploadsPer5h:   20,
	MaxUploadsPerWeek: 100,
}

var proPlan = struct {
	StorageQuotaBytes int64
	MaxFileSizeBytes  int64
	MaxUploadsPer5h   int64
	MaxUploadsPerWeek int64
}{
	StorageQuotaBytes: 50 * 1 << 30, // 50GB
	MaxFileSizeBytes:  500 << 20,    // 500MB
	MaxUploadsPer5h:   200,
	MaxUploadsPerWeek: 1000,
}

func (s *Store) GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error) {
	// Return the existing account if present.
	existing, err := s.q().GetAccountByUser(ctx, userID)
	if err == nil {
		return s.GetAccount(ctx, existing.AccountID)
	}

	// Otherwise create a new account.
	accountID := newID()
	createdAt := nowTime()
	row, err := s.q().GetOrCreateAccount(ctx, sqlcgen.GetOrCreateAccountParams{
		AccountID:          accountID,
		Name:               fmt.Sprintf("account-%s", userID),
		Plan:               "free",
		StorageQuotaBytes:  defaultFreePlan.StorageQuotaBytes,
		MaxFileSizeBytes:   defaultFreePlan.MaxFileSizeBytes,
		MaxUploadsPer5h:    int32(defaultFreePlan.MaxUploadsPer5h),
		MaxUploadsPer1week: int32(defaultFreePlan.MaxUploadsPerWeek),
		CreatedAt:          createdAt,
	})
	if err != nil {
		return nil, err
	}

	_ = s.q().CreateAccountUser(ctx, sqlcgen.CreateAccountUserParams{
		AccountID: row.AccountID,
		UserID:    userID,
		Role:      "owner",
		JoinedAt:  createdAt,
	})

	return s.GetAccount(ctx, row.AccountID)
}

func (s *Store) GetAccount(ctx context.Context, accountID string) (*domain.Account, error) {
	account, err := s.scanAccount(ctx, `
SELECT account_id, name, plan, storage_quota_bytes, storage_used_bytes, max_file_size_bytes,
       max_uploads_per_5h, max_uploads_per_1week, stripe_customer_id, stripe_subscription_id,
       billing_status, stripe_price_id, billing_currency, billing_amount_minor, billing_interval,
       current_period_end, cancel_at_period_end, billing_updated_at, created_at
FROM accounts
WHERE account_id = $1
`, accountID)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (s *Store) IsAccountAccessible(ctx context.Context, accountID, userID string) bool {
	accessible, err := s.q().IsAccountAccessible(ctx, sqlcgen.IsAccountAccessibleParams{
		AccountID: accountID,
		UserID:    userID,
	})
	return err == nil && accessible
}

func (s *Store) SetAccountStripeCustomerID(ctx context.Context, accountID, stripeCustomerID string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE accounts
SET stripe_customer_id = $2, updated_at = $3
WHERE account_id = $1
`, accountID, stripeCustomerID, nowTime())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error {
	limits, err := billingLimits(plan)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE accounts
SET plan = $2,
    storage_quota_bytes = $3,
    max_file_size_bytes = $4,
    max_uploads_per_5h = $5,
    max_uploads_per_1week = $6,
    stripe_customer_id = CASE WHEN $7 = '' THEN stripe_customer_id ELSE $7 END,
    stripe_subscription_id = $8,
    billing_status = CASE WHEN $2 = 'free' THEN 'free' ELSE 'active' END,
    billing_updated_at = $9,
    updated_at = $9
WHERE account_id = $1
`, accountID, string(plan), limits.StorageQuotaBytes, limits.MaxFileSizeBytes, limits.MaxUploadsPer5h, limits.MaxUploadsPerWeek, stripeCustomerID, stripeSubscriptionID, nowTime())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ApplyBillingPlanByStripeCustomerID(ctx context.Context, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error {
	if stripeCustomerID == "" {
		return domain.ErrNotFound
	}
	limits, err := billingLimits(plan)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE accounts
SET plan = $2,
    storage_quota_bytes = $3,
    max_file_size_bytes = $4,
    max_uploads_per_5h = $5,
    max_uploads_per_1week = $6,
    stripe_subscription_id = $7,
    billing_status = CASE WHEN $2 = 'free' THEN 'free' ELSE 'active' END,
    billing_updated_at = $8,
    updated_at = $8
WHERE stripe_customer_id = $1
`, stripeCustomerID, string(plan), limits.StorageQuotaBytes, limits.MaxFileSizeBytes, limits.MaxUploadsPer5h, limits.MaxUploadsPerWeek, stripeSubscriptionID, nowTime())
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) RecordBillingWebhookEvent(ctx context.Context, event *domain.ProviderWebhookEvent) (bool, error) {
	if event == nil || event.EventID == "" {
		return true, nil
	}
	provider := event.Provider
	if provider == "" {
		provider = "stripe"
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO billing_events (
  provider, event_id, event_type, received_at, processing_status,
  account_id, stripe_customer_id, stripe_subscription_id
)
VALUES ($1, $2, $3, $4, 'received', $5, $6, $7)
ON CONFLICT (provider, event_id) DO NOTHING
`, provider, event.EventID, event.EventType, nowTime(), event.AccountID, event.ExternalCustomerID, event.ExternalSubscriptionID)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (s *Store) MarkBillingWebhookEventProcessed(ctx context.Context, provider, eventID, status, errorMessage string) error {
	if eventID == "" {
		return nil
	}
	if provider == "" {
		provider = "stripe"
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE billing_events
SET processing_status = $3,
    error_message = $4,
    processed_at = $5
WHERE provider = $1 AND event_id = $2
`, provider, eventID, status, errorMessage, nowTime())
	return err
}

func (s *Store) ApplyBillingEvent(ctx context.Context, event *domain.ProviderWebhookEvent) error {
	if event == nil || event.Plan == "" {
		return nil
	}
	limits, err := billingLimits(event.Plan)
	if err != nil {
		return err
	}
	status := string(event.Status)
	if status == "" {
		if event.Plan == domain.BillingPlanFree {
			status = string(domain.BillingStatusFree)
		} else {
			status = string(domain.BillingStatusActive)
		}
	}
	periodEnd, err := parseBillingTime(event.CurrentPeriodEnd)
	if err != nil {
		return err
	}
	now := nowTime()
	query := `
UPDATE accounts
SET plan = $3,
    storage_quota_bytes = $4,
    max_file_size_bytes = $5,
    max_uploads_per_5h = $6,
    max_uploads_per_1week = $7,
    stripe_customer_id = CASE WHEN $8 = '' THEN stripe_customer_id ELSE $8 END,
    stripe_subscription_id = $9,
    billing_status = $10,
    stripe_price_id = $11,
    billing_currency = $12,
    billing_amount_minor = $13,
    billing_interval = $14,
    current_period_end = $15,
    cancel_at_period_end = $16,
    billing_updated_at = $17,
    updated_at = $17
WHERE `
	var res sql.Result
	if event.AccountID != "" {
		res, err = s.db.ExecContext(ctx, query+"account_id = $1", event.AccountID, "", string(event.Plan), limits.StorageQuotaBytes, limits.MaxFileSizeBytes, limits.MaxUploadsPer5h, limits.MaxUploadsPerWeek, event.ExternalCustomerID, event.ExternalSubscriptionID, status, event.ExternalPriceID, string(event.Currency), event.AmountMinor, string(event.Interval), periodEnd, event.CancelAtPeriodEnd, now)
	} else {
		res, err = s.db.ExecContext(ctx, query+"stripe_customer_id = $2", "", event.ExternalCustomerID, string(event.Plan), limits.StorageQuotaBytes, limits.MaxFileSizeBytes, limits.MaxUploadsPer5h, limits.MaxUploadsPerWeek, event.ExternalCustomerID, event.ExternalSubscriptionID, status, event.ExternalPriceID, string(event.Currency), event.AmountMinor, string(event.Interval), periodEnd, event.CancelAtPeriodEnd, now)
	}
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ListWorkspacesByUser(ctx context.Context, userID string) []*domain.Workspace {
	rows, err := s.db.QueryContext(ctx, `
SELECT w.workspace_id, w.account_id, w.name, w.created_at,
       a.plan, a.storage_used_bytes, a.storage_quota_bytes, a.max_file_size_bytes,
       a.max_uploads_per_5h, a.max_uploads_per_1week
FROM workspaces w
JOIN account_users au ON au.account_id = w.account_id
JOIN accounts a ON a.account_id = w.account_id
WHERE au.user_id = $1
ORDER BY w.created_at DESC
`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var workspaces []*domain.Workspace
	for rows.Next() {
		ws, err := scanWorkspace(rows)
		if err != nil {
			return nil
		}
		ws.RootItemID, _ = s.GetWorkspaceRootItemIDByWorkspace(ctx, ws.WorkspaceID)
		workspaces = append(workspaces, ws)
	}
	return workspaces
}

func (s *Store) GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT w.workspace_id, w.account_id, w.name, w.created_at,
       a.plan, a.storage_used_bytes, a.storage_quota_bytes, a.max_file_size_bytes,
       a.max_uploads_per_5h, a.max_uploads_per_1week
FROM workspaces w
JOIN accounts a ON a.account_id = w.account_id
WHERE w.workspace_id = $1
`, id)
	ws, err := scanWorkspace(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	ws.RootItemID, _ = s.GetWorkspaceRootItemIDByWorkspace(ctx, id)
	return ws, nil
}

func (s *Store) IsWorkspaceAccessible(ctx context.Context, wsID, userID string) bool {
	accessible, err := s.q().IsWorkspaceAccessible(ctx, sqlcgen.IsWorkspaceAccessibleParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	return err == nil && accessible
}

func (s *Store) CreateWorkspace(ctx context.Context, accountID, name string) *domain.Workspace {
	createdAt := nowTime()
	wsID := newID()
	rootItemID := newID()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil
	}
	defer tx.Rollback()
	qtx := s.q().WithTx(tx)

	if err := qtx.CreateWorkspace(ctx, sqlcgen.CreateWorkspaceParams{
		WorkspaceID: wsID,
		AccountID:   accountID,
		Name:        name,
		CreatedAt:   createdAt,
	}); err != nil {
		return nil
	}
	if err := qtx.CreateItem(ctx, sqlcgen.CreateItemParams{
		ID:          rootItemID,
		WorkspaceID: wsID,
		ParentID:    sql.NullString{},
		Title:       name,
		Level:       0,
		Description: "Workspace root",
		Content:     "",
		CreatedBy:   "system",
		CreatedAt:   createdAt,
	}); err != nil {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return nil
	}
	ws, err := s.GetWorkspace(ctx, wsID)
	if err != nil {
		return &domain.Workspace{
			WorkspaceID: wsID,
			AccountID:   accountID,
			Name:        name,
			RootItemID:  rootItemID,
			CreatedAt:   createdAt.Format(time.RFC3339),
		}
	}
	ws.RootItemID = rootItemID
	return ws
}

func (s *Store) GetWorkspaceRootItemIDByWorkspace(ctx context.Context, workspaceID string) (string, error) {
	row, err := s.q().GetTreeRoot(ctx, workspaceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return row.ID, nil
}

func toAccount(row sqlcgen.Account) *domain.Account {
	return &domain.Account{
		AccountID:            row.AccountID,
		Name:                 row.Name,
		Plan:                 row.Plan,
		StorageQuotaBytes:    row.StorageQuotaBytes,
		StorageUsedBytes:     row.StorageUsedBytes,
		MaxFileSizeBytes:     row.MaxFileSizeBytes,
		MaxUploadsPerFiveH:   int64(row.MaxUploadsPer5h),
		MaxUploadsPerWeek:    int64(row.MaxUploadsPer1week),
		StripeCustomerID:     row.StripeCustomerID,
		StripeSubscriptionID: row.StripeSubscriptionID,
		CreatedAt:            row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toWorkspace(row sqlcgen.Workspace) *domain.Workspace {
	return &domain.Workspace{
		WorkspaceID: row.WorkspaceID,
		AccountID:   row.AccountID,
		Name:        row.Name,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanAccount(ctx context.Context, query string, args ...any) (*domain.Account, error) {
	var account domain.Account
	var maxUploadsPer5h int32
	var maxUploadsPerWeek int32
	var currentPeriodEnd sql.NullTime
	var billingUpdatedAt sql.NullTime
	var createdAt time.Time
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&account.AccountID,
		&account.Name,
		&account.Plan,
		&account.StorageQuotaBytes,
		&account.StorageUsedBytes,
		&account.MaxFileSizeBytes,
		&maxUploadsPer5h,
		&maxUploadsPerWeek,
		&account.StripeCustomerID,
		&account.StripeSubscriptionID,
		&account.BillingStatus,
		&account.StripePriceID,
		&account.BillingCurrency,
		&account.BillingAmountMinor,
		&account.BillingInterval,
		&currentPeriodEnd,
		&account.CancelAtPeriodEnd,
		&billingUpdatedAt,
		&createdAt,
	); err != nil {
		return nil, err
	}
	account.MaxUploadsPerFiveH = int64(maxUploadsPer5h)
	account.MaxUploadsPerWeek = int64(maxUploadsPerWeek)
	account.CurrentPeriodEnd = formatNullTime(currentPeriodEnd)
	account.BillingUpdatedAt = formatNullTime(billingUpdatedAt)
	account.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &account, nil
}

func scanWorkspace(row scanner) (*domain.Workspace, error) {
	var ws domain.Workspace
	var createdAt time.Time
	var maxUploadsPer5h int32
	var maxUploadsPerWeek int32
	if err := row.Scan(
		&ws.WorkspaceID,
		&ws.AccountID,
		&ws.Name,
		&createdAt,
		&ws.Plan,
		&ws.StorageUsedBytes,
		&ws.StorageQuotaBytes,
		&ws.MaxFileSizeBytes,
		&maxUploadsPer5h,
		&maxUploadsPerWeek,
	); err != nil {
		return nil, err
	}
	ws.MaxUploadsPerFiveH = int64(maxUploadsPer5h)
	ws.MaxUploadsPerWeek = int64(maxUploadsPerWeek)
	ws.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &ws, nil
}

func parseBillingTime(value string) (sql.NullTime, error) {
	if value == "" {
		return sql.NullTime{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return sql.NullTime{}, err
	}
	return sql.NullTime{Time: parsed.UTC(), Valid: true}, nil
}

func formatNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func billingLimits(plan domain.BillingPlan) (struct {
	StorageQuotaBytes int64
	MaxFileSizeBytes  int64
	MaxUploadsPer5h   int64
	MaxUploadsPerWeek int64
}, error) {
	switch plan {
	case domain.BillingPlanFree:
		return defaultFreePlan, nil
	case domain.BillingPlanPro:
		return proPlan, nil
	default:
		return struct {
			StorageQuotaBytes int64
			MaxFileSizeBytes  int64
			MaxUploadsPer5h   int64
			MaxUploadsPerWeek int64
		}{}, plan.Validate()
	}
}
