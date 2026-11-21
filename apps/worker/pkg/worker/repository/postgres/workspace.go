package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/synthify/backend/internal/db/sqlcgen"
	"github.com/synthify/backend/internal/domain"
)

// defaultFreePlan defines the default plan limits for newly created accounts.
var defaultFreePlan = struct {
	StorageQuotaBytes int64
	MaxFileSizeBytes  int64
	MaxUploadsPer5h   int64
	MaxUploadsPerWeek int64
}{
	StorageQuotaBytes: 500 << 20, // 500MB
	MaxFileSizeBytes:  100 << 20, // 100MB
	MaxUploadsPer5h:   20,
	MaxUploadsPerWeek: 100,
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

// CreateWorkspace creates a new workspace record. The root of the tree is the workspace itself;
// no separate workspace_root tree item is created. Document processing inserts items directly
// beneath the workspace with a nil parent_id.
func (s *Store) CreateWorkspace(ctx context.Context, accountID, name string) (*domain.Workspace, error) {
	createdAt := nowTime()
	wsID := newID()

	if err := s.q().CreateWorkspace(ctx, sqlcgen.CreateWorkspaceParams{
		WorkspaceID: wsID,
		AccountID:   accountID,
		Name:        name,
		CreatedAt:   createdAt,
	}); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	ws, err := s.GetWorkspace(ctx, wsID)
	if err != nil {
		return &domain.Workspace{
			WorkspaceID: wsID,
			AccountID:   accountID,
			Name:        name,
			CreatedAt:   createdAt.Format(time.RFC3339),
		}, nil
	}
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

// scanner abstracts database row scanning across sql.Row and sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanAccount executes an account query and scans the result into an Account domain object.
func (s *Store) scanAccount(ctx context.Context, query string, args ...any) (*domain.Account, error) {
	return scanAccountRow(s.db.QueryRowContext(ctx, query, args...))
}

// scanAccountRow parses database columns into a populated domain Account.
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

// scanWorkspace parses database columns into a populated domain Workspace.
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

// formatNullTime formats a valid sql.NullTime as an RFC3339 string, returning empty if null.
func formatNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}
