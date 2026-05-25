package service

import (
	"context"
	"log/slog"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// ItemUsecase は handler が依存する ItemService の API 表面。
// 各メソッドは userID を受け取り、内部で workspace authz を実施する。
type ItemUsecase interface {
	GetItem(ctx context.Context, itemID, workspaceID, userID string) (*domain.Item, error)
	CreateItem(ctx context.Context, workspaceID, label, description, parentID, userID string) (*domain.Item, error)
	ApproveAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error
	RejectAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error
}

type ItemService struct {
	repo       repository.ItemRepository
	tree       repository.TreeRepository
	workspaces repository.WorkspaceRepository
	logger     *slog.Logger
}

func NewItemService(repo repository.ItemRepository, tree repository.TreeRepository, workspaces repository.WorkspaceRepository, logger *slog.Logger) *ItemService {
	return &ItemService{repo: repo, tree: tree, workspaces: workspaces, logger: logger}
}

func (s *ItemService) authorizeWorkspace(ctx context.Context, workspaceID, userID string) error {
	if !s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID) {
		return domain.ErrForbidden
	}
	return nil
}

// GetItem は item を取得する。workspace authz と item の workspace 帰属チェックを行う。
func (s *ItemService) GetItem(ctx context.Context, itemID, workspaceID, userID string) (*domain.Item, error) {
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	item, err := s.repo.GetItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item.WorkspaceID != workspaceID {
		return nil, domain.ErrForbidden
	}
	return item, nil
}

func (s *ItemService) CreateItem(ctx context.Context, workspaceID, label, description, parentID, userID string) (*domain.Item, error) {
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	if _, err := s.tree.GetOrCreateTree(ctx, workspaceID); err != nil {
		return nil, err
	}
	item := s.repo.CreateItem(ctx, workspaceID, label, description, parentID, userID)
	if item == nil {
		return nil, domain.ErrNotFound
	}
	return item, nil
}

func (s *ItemService) ApproveAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error {
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.repo.ApproveAlias(ctx, workspaceID, canonicalItemID, aliasItemID)
}

func (s *ItemService) RejectAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error {
	if err := s.authorizeWorkspace(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.repo.RejectAlias(ctx, workspaceID, canonicalItemID, aliasItemID)
}
