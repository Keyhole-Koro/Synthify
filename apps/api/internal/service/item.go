package service

import (
	"context"
	"log/slog"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

type ItemService struct {
	repo       repository.ItemRepository
	workspaces repository.WorkspaceRepository
	logger     *slog.Logger
}

type ItemServiceDeps struct {
	Repo       repository.ItemRepository
	Workspaces repository.WorkspaceRepository
	Logger     *slog.Logger
}

func NewItemService(deps ItemServiceDeps) *ItemService {
	return &ItemService{
		repo:       deps.Repo,
		workspaces: deps.Workspaces,
		logger:     deps.Logger,
	}
}

func (s *ItemService) authorizeWorkspace(ctx context.Context, workspaceID, userID string) error {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

// authorizeWrite は書き込み操作の権限を確認する。viewer は read のみ許可され、
// editor / owner だけが書き込める。非メンバーも Forbidden。
func (s *ItemService) authorizeWrite(ctx context.Context, workspaceID, userID string) error {
	role, err := s.workspaces.GetWorkspaceRole(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !role.CanWrite() {
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

func (s *ItemService) GetItemEvidence(ctx context.Context, itemID, workspaceID, userID string) (*domain.ItemEvidence, error) {
	item, err := s.GetItem(ctx, itemID, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	sources, err := s.repo.ListItemSources(ctx, item.ItemID)
	if err != nil {
		return nil, err
	}
	sourceRefs, err := s.repo.ListItemSourceRefs(ctx, item.ItemID)
	if err != nil {
		return nil, err
	}
	return &domain.ItemEvidence{
		Sources:    sources,
		SourceRefs: sourceRefs,
	}, nil
}

func (s *ItemService) CreateItem(ctx context.Context, workspaceID, label, description, parentID, userID string) (*domain.Item, error) {
	if err := s.authorizeWrite(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.repo.CreateItem(ctx, workspaceID, label, description, parentID, userID)
}

func (s *ItemService) ApproveAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error {
	if err := s.authorizeWrite(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.repo.ApproveAlias(ctx, workspaceID, canonicalItemID, aliasItemID)
}

func (s *ItemService) RejectAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error {
	if err := s.authorizeWrite(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.repo.RejectAlias(ctx, workspaceID, canonicalItemID, aliasItemID)
}
