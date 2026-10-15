package postgres

import (
	"context"
	"database/sql"
	"errors"
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

// proPlan defines the resource quotas and upload rate limits for pro/usage-based accounts.
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

// GetOrCreateAccount retrieves the account associated with the given user ID,
// or creates a new free-tier account if one does not exist yet.
func (s *Store) GetOrCreateAccount(ctx context.Context, userID string) (*domain.Account, error) {
	// Return the existing account if present.
	existing, err := s.GetAccountByUser(ctx, userID)
	if err == nil {
		return existing, nil
	}

	// Otherwise create a new account.
	return s.CreateAccount(ctx, userID)
}

// GetAccountByUser retrieves the account associated with the specified user ID.
// Returns domain.ErrNotFound if no account is found.
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

// CreateAccount provisions a new account for the specified user with default free-tier quotas
// and assigns the user as the account owner.
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

// GetAccount retrieves the account with the specified account ID.
// Returns domain.ErrNotFound if the account does not exist.
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

// IsAccountAccessible checks if the given user is a member of the specified account.
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

// SetAccountStripeCustomerID associates a Stripe customer ID with the specified account.
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

// ListStripeLinkedAccounts returns a list of accounts that have a non-empty Stripe customer ID,
// ordered by update time ascending, up to the given limit.
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

// ApplyBillingPlan updates an account's billing plan, resource limits, and Stripe details by account ID.
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

// ApplyBillingPlanByStripeCustomerID updates an account's plan and quotas using its Stripe customer ID.
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

// RecordBillingWebhookEvent logs an incoming billing webhook event in billing_events for idempotency.
// Returns true if this is a newly inserted event, or false if it was already recorded.
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

// MarkBillingWebhookEventProcessed updates the status and optional error message of a billing webhook event.
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

// ApplyBillingEvent updates account billing status, subscription, pricing, and limits from a webhook event.
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

// ListWorkspacesByUser returns all non-deleted workspaces accessible to the specified user.
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

// GetWorkspace retrieves a workspace by its ID, including account quotas.
// Returns domain.ErrNotFound if the workspace does not exist or has been soft-deleted.
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

// IsWorkspaceAccessible checks if the user has access to the specified workspace,
// either via direct workspace membership or account ownership.
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

// GetWorkspaceRole retrieves the user's role within the specified workspace.
func (s *Store) GetWorkspaceRole(ctx context.Context, wsID, userID string) (domain.WorkspaceRole, error) {
	role, err := s.q().GetWorkspaceRole(ctx, sqlcgen.GetWorkspaceRoleParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	if err != nil {
		return "", fmt.Errorf("query workspace role: %w", err)
	}
	return domain.WorkspaceRole(role), nil
}

// UpsertWorkspaceMember adds a user to a workspace or updates their role if already a member.
func (s *Store) UpsertWorkspaceMember(ctx context.Context, wsID, userID string, role domain.WorkspaceRole, invitedBy string) error {
	if err := s.q().UpsertWorkspaceMember(ctx, sqlcgen.UpsertWorkspaceMemberParams{
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        string(role),
		InvitedBy:   invitedBy,
		InvitedAt:   nowTime(),
	}); err != nil {
		return fmt.Errorf("upsert workspace member: %w", err)
	}
	return nil
}

// ListWorkspaceMembers returns all members and their roles for the specified workspace.
func (s *Store) ListWorkspaceMembers(ctx context.Context, wsID string) ([]*domain.WorkspaceMember, error) {
	rows, err := s.q().ListWorkspaceMembers(ctx, wsID)
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}
	members := make([]*domain.WorkspaceMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, &domain.WorkspaceMember{
			WorkspaceID: row.WorkspaceID,
			UserID:      row.UserID,
			Email:       row.Email,
			Role:        domain.WorkspaceRole(row.Role),
			InvitedBy:   row.InvitedBy,
			InvitedAt:   row.InvitedAt.UTC().Format(time.RFC3339),
		})
	}
	return members, nil
}

// RemoveWorkspaceMember removes a member from the specified workspace.
// Returns domain.ErrNotFound if the member was not part of the workspace.
func (s *Store) RemoveWorkspaceMember(ctx context.Context, wsID, userID string) error {
	affected, err := s.q().RemoveWorkspaceMember(ctx, sqlcgen.RemoveWorkspaceMemberParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	if err != nil {
		return fmt.Errorf("remove workspace member: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CreateShareLink generates an invitation share link for a workspace with an expiration time.
func (s *Store) CreateShareLink(ctx context.Context, link *domain.ShareLink) error {
	expiresAt, err := parseBillingTime(link.ExpiresAt)
	if err != nil {
		return fmt.Errorf("parse share link expires_at: %w", err)
	}
	if err := s.q().CreateShareLink(ctx, sqlcgen.CreateShareLinkParams{
		Token:       link.Token,
		WorkspaceID: link.WorkspaceID,
		Role:        string(link.Role),
		CreatedBy:   link.CreatedBy,
		ExpiresAt:   expiresAt,
		CreatedAt:   nowTime(),
	}); err != nil {
		return fmt.Errorf("create share link: %w", err)
	}
	return nil
}

// ListShareLinks returns all active and revoked share links for the workspace.
func (s *Store) ListShareLinks(ctx context.Context, wsID string) ([]*domain.ShareLink, error) {
	rows, err := s.q().ListShareLinks(ctx, wsID)
	if err != nil {
		return nil, fmt.Errorf("list share links: %w", err)
	}
	links := make([]*domain.ShareLink, 0, len(rows))
	for _, row := range rows {
		links = append(links, toDomainShareLink(row))
	}
	return links, nil
}

// RevokeShareLink marks a share link token as revoked.
// Returns domain.ErrNotFound if no such token exists.
func (s *Store) RevokeShareLink(ctx context.Context, wsID, token string, now time.Time) error {
	affected, err := s.q().RevokeShareLink(ctx, sqlcgen.RevokeShareLinkParams{
		WorkspaceID: wsID,
		Token:       token,
		RevokedAt:   sql.NullTime{Time: now.UTC(), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("revoke share link: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ResolveShareLink retrieves an unexpired, non-revoked share link by its token.
// Returns domain.ErrNotFound if the token does not exist or has expired.
func (s *Store) ResolveShareLink(ctx context.Context, token string, now time.Time) (*domain.ShareLink, error) {
	row, err := s.q().ResolveShareLink(ctx, sqlcgen.ResolveShareLinkParams{
		Token:     token,
		ExpiresAt: sql.NullTime{Time: now.UTC(), Valid: true},
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("resolve share link: %w", err)
	}
	return toDomainShareLink(row), nil
}

// toDomainShareLink converts a sqlcgen.WorkspaceShareLink row into a domain.ShareLink.
func toDomainShareLink(row sqlcgen.WorkspaceShareLink) *domain.ShareLink {
	return &domain.ShareLink{
		Token:       row.Token,
		WorkspaceID: row.WorkspaceID,
		Role:        domain.WorkspaceRole(row.Role),
		CreatedBy:   row.CreatedBy,
		ExpiresAt:   formatNullTime(row.ExpiresAt),
		Revoked:     row.RevokedAt.Valid,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// CreateWorkspace inserts a workspace row.
// It does not wrap the operation in a transaction; if atomicity is required,
// wrap the caller in Transactor.WithTx.
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
	// No workspace_root tree_item is created: the workspace itself is the tree
	// root. Knowledge nodes are inserted directly under it (parent_id IS NULL)
	// when a document is processed.
	ws, err := s.GetWorkspace(ctx, wsID)
	if err != nil {
		// Return constructed domain object if reread fails immediately after write.
		return &domain.Workspace{
			WorkspaceID: wsID,
			AccountID:   accountID,
			Name:        name,
			CreatedAt:   createdAt.Format(time.RFC3339),
		}, nil
	}
	return ws, nil
}

// UpdateWorkspaceName updates the display name of a workspace.
// Returns domain.ErrNotFound if the workspace does not exist.
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

// DeleteWorkspace marks a workspace as deleted via soft deletion.
// Returns domain.ErrNotFound if the workspace does not exist.
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

// toAccount converts a sqlcgen.Account query row into a domain.Account entity.
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

// toWorkspace converts a sqlcgen.Workspace query row into a domain.Workspace entity.
func toWorkspace(row sqlcgen.Workspace) *domain.Workspace {
	return &domain.Workspace{
		WorkspaceID: row.WorkspaceID,
		AccountID:   row.AccountID,
		Name:        row.Name,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// scanner defines an abstraction over sql.Row and sql.Rows for scanning values.
type scanner interface {
	Scan(dest ...any) error
}

// scanAccount executes an account query and scans the result into a domain.Account.
func (s *Store) scanAccount(ctx context.Context, query string, args ...any) (*domain.Account, error) {
	return scanAccountRow(s.db.QueryRowContext(ctx, query, args...))
}

// scanAccountRow scans a single database row into a domain.Account struct.
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

// scanWorkspace scans a single database row into a domain.Workspace struct.
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

// parseBillingTime converts an RFC3339 timestamp string into a nullable sql.NullTime.
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

// formatNullTime formats a nullable sql.NullTime into an RFC3339 string, or returns empty string if null.
func formatNullTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

// billingLimits returns the default quota and rate limit configurations for the given billing plan.
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
