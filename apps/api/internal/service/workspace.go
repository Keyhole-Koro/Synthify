package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// WorkspaceUsecase は handler が依存する WorkspaceService の API 表面。
// ListWorkspaces は PR 4 (認可 Service 移動) で handler の直接 repository call を
// この interface 経由に切り替える前提でここに含める。
type WorkspaceUsecase interface {
	ListWorkspaces(ctx context.Context, userID string) []*domain.Workspace
	GetWorkspace(ctx context.Context, id, userID string) (*domain.Workspace, error)
	CreateWorkspace(ctx context.Context, name, userID string) (*domain.Workspace, error)
	UpdateWorkspace(ctx context.Context, id, name, userID string) (*domain.Workspace, error)
}

type WorkspaceService struct {
	accounts   repository.AccountRepository
	workspaces repository.WorkspaceRepository
	logger     *slog.Logger
}

func NewWorkspaceService(accounts repository.AccountRepository, workspaces repository.WorkspaceRepository, logger *slog.Logger) *WorkspaceService {
	return &WorkspaceService{accounts: accounts, workspaces: workspaces, logger: logger}
}

func (s *WorkspaceService) ListWorkspaces(ctx context.Context, userID string) []*domain.Workspace {
	return s.workspaces.ListWorkspacesByUser(ctx, userID)
}

func (s *WorkspaceService) GetWorkspace(ctx context.Context, id, userID string) (*domain.Workspace, error) {
	if !s.workspaces.IsWorkspaceAccessible(ctx, id, userID) {
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
	ws := s.workspaces.CreateWorkspace(ctx, account.AccountID, name)
	if ws == nil {
		return nil, errors.New("failed to create workspace")
	}
	return ws, nil
}

func (s *WorkspaceService) UpdateWorkspace(ctx context.Context, id, name, userID string) (*domain.Workspace, error) {
	if !s.workspaces.IsWorkspaceAccessible(ctx, id, userID) {
		return nil, domain.ErrForbidden
	}
	ws, err := s.workspaces.UpdateWorkspaceName(ctx, id, name)
	if err != nil {
		return nil, err
	}
	return ws, nil
}
