package postgres

import (
	"context"
	"database/sql"
	"errors"
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
	row, err := s.q().GetAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get account: %w", err)
	}
	return accountFromGetAccountRow(row), nil
}

func accountFromGetAccountRow(row sqlcgen.GetAccountRow) *domain.Account {
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
		BillingStatus:        row.BillingStatus,
		StripePriceID:        row.StripePriceID,
		BillingCurrency:      row.BillingCurrency,
		BillingAmountMinor:   row.BillingAmountMinor,
		BillingInterval:      row.BillingInterval,
		CurrentPeriodEnd:     formatNullTime(row.CurrentPeriodEnd),
		CancelAtPeriodEnd:    row.CancelAtPeriodEnd,
		BillingUpdatedAt:     formatNullTime(row.BillingUpdatedAt),
		CreatedAt:            row.CreatedAt.UTC().Format(time.RFC3339),
	}
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
	rows, err := s.q().SetAccountStripeCustomerID(ctx, sqlcgen.SetAccountStripeCustomerIDParams{
		AccountID:        accountID,
		StripeCustomerID: stripeCustomerID,
		UpdatedAt:        nowTime(),
	})
	if err != nil {
		return fmt.Errorf("set account stripe customer id: %w", err)
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
	rows, err := s.q().ListStripeLinkedAccounts(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("list stripe linked accounts: %w", err)
	}
	accounts := make([]*domain.Account, 0, len(rows))
	for _, row := range rows {
		accounts = append(accounts, accountFromStripeLinkedRow(row))
	}
	return accounts, nil
}

func accountFromStripeLinkedRow(row sqlcgen.ListStripeLinkedAccountsRow) *domain.Account {
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
		BillingStatus:        row.BillingStatus,
		StripePriceID:        row.StripePriceID,
		BillingCurrency:      row.BillingCurrency,
		BillingAmountMinor:   row.BillingAmountMinor,
		BillingInterval:      row.BillingInterval,
		CurrentPeriodEnd:     formatNullTime(row.CurrentPeriodEnd),
		CancelAtPeriodEnd:    row.CancelAtPeriodEnd,
		BillingUpdatedAt:     formatNullTime(row.BillingUpdatedAt),
		CreatedAt:            row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *Store) ApplyBillingPlan(ctx context.Context, accountID, stripeCustomerID, stripeSubscriptionID string, plan domain.BillingPlan) error {
	limits, err := billingLimits(plan)
	if err != nil {
		return err
	}
	rows, err := s.q().ApplyBillingPlan(ctx, sqlcgen.ApplyBillingPlanParams{
		AccountID:            accountID,
		Plan:                 string(plan),
		StorageQuotaBytes:    limits.StorageQuotaBytes,
		MaxFileSizeBytes:     limits.MaxFileSizeBytes,
		MaxUploadsPer5h:      int32(limits.MaxUploadsPer5h),
		MaxUploadsPer1week:   int32(limits.MaxUploadsPerWeek),
		StripeCustomerID:     stripeCustomerID,
		StripeSubscriptionID: stripeSubscriptionID,
		Now:                  nowTime(),
	})
	if err != nil {
		return fmt.Errorf("apply billing plan: %w", err)
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
	rows, err := s.q().ApplyBillingPlanByStripeCustomerID(ctx, sqlcgen.ApplyBillingPlanByStripeCustomerIDParams{
		StripeCustomerID:     stripeCustomerID,
		Plan:                 string(plan),
		StorageQuotaBytes:    limits.StorageQuotaBytes,
		MaxFileSizeBytes:     limits.MaxFileSizeBytes,
		MaxUploadsPer5h:      int32(limits.MaxUploadsPer5h),
		MaxUploadsPer1week:   int32(limits.MaxUploadsPerWeek),
		StripeSubscriptionID: stripeSubscriptionID,
		Now:                  nowTime(),
	})
	if err != nil {
		return fmt.Errorf("apply billing plan by stripe customer id: %w", err)
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
	rows, err := s.q().RecordBillingWebhookEvent(ctx, sqlcgen.RecordBillingWebhookEventParams{
		Provider:             provider,
		EventID:              event.EventID,
		EventType:            event.EventType,
		ReceivedAt:           nowTime(),
		AccountID:            event.AccountID,
		StripeCustomerID:     event.ExternalCustomerID,
		StripeSubscriptionID: event.ExternalSubscriptionID,
	})
	if err != nil {
		return false, fmt.Errorf("record billing webhook event: %w", err)
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
	if err := s.q().MarkBillingWebhookEventProcessed(ctx, sqlcgen.MarkBillingWebhookEventProcessedParams{
		Provider:         provider,
		EventID:          eventID,
		ProcessingStatus: status,
		ErrorMessage:     errorMessage,
		ProcessedAt:      sql.NullTime{Time: nowTime(), Valid: true},
	}); err != nil {
		return fmt.Errorf("mark billing webhook event processed: %w", err)
	}
	return nil
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
	var rows int64
	if event.AccountID != "" {
		rows, err = s.q().ApplyBillingEventByAccount(ctx, sqlcgen.ApplyBillingEventByAccountParams{
			Plan:                 string(event.Plan),
			StorageQuotaBytes:    limits.StorageQuotaBytes,
			MaxFileSizeBytes:     limits.MaxFileSizeBytes,
			MaxUploadsPer5h:      int32(limits.MaxUploadsPer5h),
			MaxUploadsPer1week:   int32(limits.MaxUploadsPerWeek),
			StripeCustomerID:     event.ExternalCustomerID,
			StripeSubscriptionID: event.ExternalSubscriptionID,
			BillingStatus:        status,
			StripePriceID:        event.ExternalPriceID,
			BillingCurrency:      string(event.Currency),
			BillingAmountMinor:   event.AmountMinor,
			BillingInterval:      string(event.Interval),
			CurrentPeriodEnd:     periodEnd,
			CancelAtPeriodEnd:    event.CancelAtPeriodEnd,
			Now:                  now,
			AccountID:            event.AccountID,
		})
	} else {
		rows, err = s.q().ApplyBillingEventByStripeCustomer(ctx, sqlcgen.ApplyBillingEventByStripeCustomerParams{
			Plan:                      string(event.Plan),
			StorageQuotaBytes:         limits.StorageQuotaBytes,
			MaxFileSizeBytes:          limits.MaxFileSizeBytes,
			MaxUploadsPer5h:           int32(limits.MaxUploadsPer5h),
			MaxUploadsPer1week:        int32(limits.MaxUploadsPerWeek),
			StripeCustomerIDPassthru:  event.ExternalCustomerID,
			StripeSubscriptionID:      event.ExternalSubscriptionID,
			BillingStatus:             status,
			StripePriceID:             event.ExternalPriceID,
			BillingCurrency:           string(event.Currency),
			BillingAmountMinor:        event.AmountMinor,
			BillingInterval:           string(event.Interval),
			CurrentPeriodEnd:          periodEnd,
			CancelAtPeriodEnd:         event.CancelAtPeriodEnd,
			Now:                       now,
			StripeCustomerIDMatch:     event.ExternalCustomerID,
		})
	}
	if err != nil {
		return fmt.Errorf("apply billing event: %w", err)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) ListWorkspacesByUser(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	rows, err := s.q().ListWorkspacesByUserWithAccount(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces by user: %w", err)
	}
	workspaces := make([]*domain.Workspace, 0, len(rows))
	for _, row := range rows {
		ws := &domain.Workspace{
			WorkspaceID:        row.WorkspaceID,
			AccountID:          row.AccountID,
			Name:               row.Name,
			Plan:               row.Plan,
			StorageUsedBytes:   row.StorageUsedBytes,
			StorageQuotaBytes:  row.StorageQuotaBytes,
			MaxFileSizeBytes:   row.MaxFileSizeBytes,
			MaxUploadsPerFiveH: int64(row.MaxUploadsPer5h),
			MaxUploadsPerWeek:  int64(row.MaxUploadsPer1week),
			CreatedAt:          row.CreatedAt.UTC().Format(time.RFC3339),
		}
		ws.RootItemID, _ = s.GetWorkspaceRootItemIDByWorkspace(ctx, ws.WorkspaceID)
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

func (s *Store) GetWorkspace(ctx context.Context, id string) (*domain.Workspace, error) {
	row, err := s.q().GetWorkspaceWithAccount(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	ws := &domain.Workspace{
		WorkspaceID:        row.WorkspaceID,
		AccountID:          row.AccountID,
		Name:               row.Name,
		Plan:               row.Plan,
		StorageUsedBytes:   row.StorageUsedBytes,
		StorageQuotaBytes:  row.StorageQuotaBytes,
		MaxFileSizeBytes:   row.MaxFileSizeBytes,
		MaxUploadsPerFiveH: int64(row.MaxUploadsPer5h),
		MaxUploadsPerWeek:  int64(row.MaxUploadsPer1week),
		CreatedAt:          row.CreatedAt.UTC().Format(time.RFC3339),
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
