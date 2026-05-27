package service

import (
	"context"
	"log/slog"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// WorkspaceUsecase は handler が依存する WorkspaceService の API 表面。
// ListWorkspaces は PR 4 (認可 Service 移動) で handler の直接 repository call を
// この interface 経由に切り替える前提でここに含める。
type WorkspaceUsecase interface {
	ListWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error)
	GetWorkspace(ctx context.Context, id, userID string) (*domain.Workspace, error)
	CreateWorkspace(ctx context.Context, name, userID string) (*domain.Workspace, error)
	UpdateWorkspace(ctx context.Context, id, name, userID string) (*domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, id, userID string) error
}

type WorkspaceService struct {
	accounts   repository.AccountRepository
	workspaces repository.WorkspaceRepository
	logger     *slog.Logger
}

func NewWorkspaceService(accounts repository.AccountRepository, workspaces repository.WorkspaceRepository, logger *slog.Logger) *WorkspaceService {
	return &WorkspaceService{accounts: accounts, workspaces: workspaces, logger: logger}
}

func (s *WorkspaceService) ListWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	return s.workspaces.ListWorkspacesByUser(ctx, userID)
}

func (s *WorkspaceService) GetWorkspace(ctx context.Context, id, userID string) (*domain.Workspace, error) {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	ws, err := s.workspaces.GetWorkspace(ctx, id)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func (s *WorkspaceService) CreateWorkspace(ctx context.Context, name, userID string) (*domain.Workspace, error) {
	account, err := s.accounts.GetAccountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.workspaces.CreateWorkspace(ctx, account.AccountID, name)
}

func (s *WorkspaceService) UpdateWorkspace(ctx context.Context, id, name, userID string) (*domain.Workspace, error) {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	ws, err := s.workspaces.UpdateWorkspaceName(ctx, id, name)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func (s *WorkspaceService) DeleteWorkspace(ctx context.Context, id, userID string) error {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return s.workspaces.DeleteWorkspace(ctx, id)
}
