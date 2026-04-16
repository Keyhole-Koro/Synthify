package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Keyhole-Koro/SynthifyShared/domain"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

// defaultRegisteredPlan defines the default plan limits for newly created users.
var defaultRegisteredPlan = struct {
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

func (s *Store) GetOrCreateAccount(userID string) (*domain.Account, error) {
	ctx := context.Background()

	// Return the existing account if present.
	existing, err := s.q().GetAccountByUser(ctx, userID)
	if err == nil {
		acct := toAccount(existing)
		return acct, nil
	}

	// Otherwise create a new account.
	accountID := newID()
	createdAt := nowTime()
	row, err := s.q().GetOrCreateAccount(ctx, sqlcgen.GetOrCreateAccountParams{
		AccountID:          accountID,
		Name:               fmt.Sprintf("account-%s", userID),
		Plan:               "registered",
		StorageQuotaBytes:  defaultRegisteredPlan.StorageQuotaBytes,
		MaxFileSizeBytes:   defaultRegisteredPlan.MaxFileSizeBytes,
		MaxUploadsPer5h:    defaultRegisteredPlan.MaxUploadsPer5h,
		MaxUploadsPer1week: defaultRegisteredPlan.MaxUploadsPerWeek,
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

	return toAccount(row), nil
}

func (s *Store) GetAccount(accountID string) (*domain.Account, error) {
	row, err := s.q().GetAccount(context.Background(), accountID)
	if err != nil {
		return nil, err
	}
	return toAccount(row), nil
}

func (s *Store) ListWorkspacesByUser(userID string) []*domain.Workspace {
	rows, err := s.q().ListWorkspacesByUser(context.Background(), userID)
	if err != nil {
		return nil
	}
	var workspaces []*domain.Workspace
	for _, row := range rows {
		workspaces = append(workspaces, &domain.Workspace{
			WorkspaceID: row.WorkspaceID,
			AccountID:   row.AccountID,
			Name:        row.Name,
			CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return workspaces
}

func (s *Store) GetWorkspace(id string) (*domain.Workspace, bool) {
	row, err := s.q().GetWorkspace(context.Background(), id)
	if err != nil {
		return nil, false
	}
	return toWorkspace(row), true
}

func (s *Store) IsWorkspaceAccessible(wsID, userID string) bool {
	accessible, err := s.q().IsWorkspaceAccessible(context.Background(), sqlcgen.IsWorkspaceAccessibleParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	return err == nil && accessible
}

func (s *Store) CreateWorkspace(accountID, name string) *domain.Workspace {
	createdAt := nowTime()
	wsID := newID()
	if err := s.q().CreateWorkspace(context.Background(), sqlcgen.CreateWorkspaceParams{
		WorkspaceID: wsID,
		AccountID:   accountID,
		Name:        name,
		CreatedAt:   createdAt,
	}); err != nil {
		return nil
	}
	return &domain.Workspace{
		WorkspaceID: wsID,
		AccountID:   accountID,
		Name:        name,
		CreatedAt:   createdAt.Format(time.RFC3339),
	}
}
