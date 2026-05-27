package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/postgres/sqlcgen"
)

// defaultFreePlan defines the default plan limits for newly created accounts.
var defaultFreePlan = struct {
	StorageQuotaBytes int64
	MaxFileSizeBytes  int64
	MaxUploadsPer5h   int64
	MaxUploadsPerWeek int64
}{
	StorageQuotaBytes: 1 << 19,   // 0.5MB
	MaxFileSizeBytes:  100 << 20, // 100MB
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
	existing, err := s.GetAccountByUser(ctx, userID)
	if err == nil {
		return existing, nil
	}

	// Otherwise create a new account.
	return s.CreateAccount(ctx, userID)
}

func (s *Store) GetAccountByUser(ctx context.Context, userID string) (*domain.Account, error) {
	row, err := s.q().GetAccountByUser(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return s.GetAccount(ctx, row.AccountID)
}

func (s *Store) CreateAccount(ctx context.Context, userID string) (*domain.Account, error) {
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

func (s *Store) IsAccountAccessible(ctx context.Context, accountID, userID string) (bool, error) {
	accessible, err := s.q().IsAccountAccessible(ctx, sqlcgen.IsAccountAccessibleParams{
		AccountID: accountID,
		UserID:    userID,
	})
	if err != nil {
		return false, fmt.Errorf("query account accessibility: %w", err)
	}
	return accessible, nil
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

func (s *Store) ListStripeLinkedAccounts(ctx context.Context, limit int) ([]*domain.Account, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT account_id, name, plan, storage_quota_bytes, storage_used_bytes, max_file_size_bytes,
       max_uploads_per_5h, max_uploads_per_1week, stripe_customer_id, stripe_subscription_id,
       billing_status, stripe_price_id, billing_currency, billing_amount_minor, billing_interval,
       current_period_end, cancel_at_period_end, billing_updated_at, created_at
FROM accounts
WHERE stripe_customer_id <> ''
ORDER BY updated_at ASC
LIMIT $1
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make([]*domain.Account, 0, limit)
	for rows.Next() {
		account, err := scanAccountRow(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
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
	if event == nil {
		return nil
	}
	limits, err := billingLimits(domain.BillingPlanFree)
	if err != nil {
		return err
	}
	if event.Plan != "" {
		limits, err = billingLimits(event.Plan)
		if err != nil {
			return err
		}
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
SET plan = CASE WHEN $3 = '' THEN plan ELSE $3 END,
    storage_quota_bytes = CASE WHEN $3 = '' THEN storage_quota_bytes ELSE $4 END,
    max_file_size_bytes = CASE WHEN $3 = '' THEN max_file_size_bytes ELSE $5 END,
    max_uploads_per_5h = CASE WHEN $3 = '' THEN max_uploads_per_5h ELSE $6 END,
    max_uploads_per_1week = CASE WHEN $3 = '' THEN max_uploads_per_1week ELSE $7 END,
    stripe_customer_id = CASE WHEN $8 = '' THEN stripe_customer_id ELSE $8 END,
    stripe_subscription_id = CASE WHEN $9 = '' AND $3 <> 'free' THEN stripe_subscription_id ELSE $9 END,
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

func (s *Store) ListWorkspacesByUser(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT w.workspace_id, w.account_id, w.name, w.created_at,
       a.plan, a.storage_used_bytes, a.storage_quota_bytes, a.max_file_size_bytes,
       a.max_uploads_per_5h, a.max_uploads_per_1week
FROM workspaces w
JOIN account_users au ON au.account_id = w.account_id
JOIN accounts a ON a.account_id = w.account_id
WHERE au.user_id = $1 AND w.deleted_at IS NULL
ORDER BY w.created_at DESC
`, userID)
	if err != nil {
		return nil, fmt.Errorf("query workspaces: %w", err)
	}
	defer rows.Close()
	var workspaces []*domain.Workspace
	for rows.Next() {
		ws, err := scanWorkspace(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		ws.RootItemID, _ = s.GetWorkspaceRootItemIDByWorkspace(ctx, ws.WorkspaceID)
		workspaces = append(workspaces, ws)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspaces: %w", err)
	}
	return workspaces, nil
}

func (s *Store) GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT w.workspace_id, w.account_id, w.name, w.created_at,
       a.plan, a.storage_used_bytes, a.storage_quota_bytes, a.max_file_size_bytes,
       a.max_uploads_per_5h, a.max_uploads_per_1week
FROM workspaces w
JOIN accounts a ON a.account_id = w.account_id
WHERE w.workspace_id = $1 AND w.deleted_at IS NULL
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

func (s *Store) IsWorkspaceAccessible(ctx context.Context, wsID, userID string) (bool, error) {
	accessible, err := s.q().IsWorkspaceAccessible(ctx, sqlcgen.IsWorkspaceAccessibleParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	if err != nil {
		return false, fmt.Errorf("query workspace accessibility: %w", err)
	}
	return accessible, nil
}

// CreateWorkspace は workspaces 行と tree root item を 1 ペアで作成する。
// 内部で tx を張らないため、atomic 性が必要なら呼び出し側を
// Transactor.WithTx で包むこと。
func (s *Store) CreateWorkspace(ctx context.Context, accountID, name string) (*domain.Workspace, error) {
	createdAt := nowTime()
	wsID := newID()
	rootItemID := newID()

	if err := s.q().CreateWorkspace(ctx, sqlcgen.CreateWorkspaceParams{
		WorkspaceID: wsID,
		AccountID:   accountID,
		Name:        name,
		CreatedAt:   createdAt,
	}); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	if err := s.q().CreateItem(ctx, sqlcgen.CreateItemParams{
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
		return nil, fmt.Errorf("create workspace root item: %w", err)
	}
	ws, err := s.GetWorkspace(ctx, wsID)
	if err != nil {
		// 作成直後の読み戻しが失敗した場合は、書き込んだ値を組み立てて返す。
		// FK 等の整合性は CreateItem の段階で確認済みなので、Internal にせず
		// 組み立てた表現を返してフォールバック。
		return &domain.Workspace{
			WorkspaceID: wsID,
			AccountID:   accountID,
			Name:        name,
			RootItemID:  rootItemID,
			CreatedAt:   createdAt.Format(time.RFC3339),
		}, nil
	}
	ws.RootItemID = rootItemID
	return ws, nil
}

func (s *Store) UpdateWorkspaceName(ctx context.Context, workspaceID, name string) (*domain.Workspace, error) {
	affected, err := s.q().UpdateWorkspaceName(ctx, sqlcgen.UpdateWorkspaceNameParams{
		WorkspaceID: workspaceID,
		Name:        name,
		UpdatedAt:   nowTime(),
	})
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, domain.ErrNotFound
	}
	return s.GetWorkspace(ctx, workspaceID)
}

func (s *Store) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	affected, err := s.q().SoftDeleteWorkspace(ctx, sqlcgen.SoftDeleteWorkspaceParams{
		WorkspaceID: workspaceID,
		DeletedAt:   sql.NullTime{Time: nowTime(), Valid: true},
	})
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
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
	return scanAccountRow(s.db.QueryRowContext(ctx, query, args...))
}

func scanAccountRow(row scanner) (*domain.Account, error) {
	var account domain.Account
	var maxUploadsPer5h int32
	var maxUploadsPerWeek int32
	var currentPeriodEnd sql.NullTime
	var billingUpdatedAt sql.NullTime
	var createdAt time.Time
	if err := row.Scan(
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
	case domain.BillingPlanUsageBased:
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
