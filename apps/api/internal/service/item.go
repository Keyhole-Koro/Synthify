package service

import (
	"context"

	"github.com/synthify/backend/internal/platform/applog"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

type ItemService struct {
	repo   repository.ItemRepository
	tree   repository.TreeRepository
	logger applog.Logger
}

func NewItemService(repo repository.ItemRepository, tree repository.TreeRepository, logger applog.Logger) *ItemService {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	return &ItemService{repo: repo, tree: tree, logger: logger}
}

func (s *ItemService) CreateItem(ctx context.Context, workspaceID, label, description, parentID, createdBy string) (*domain.Item, error) {
	if _, err := s.tree.GetOrCreateTree(ctx, workspaceID); err != nil {
		return nil, err
	}
	item := s.repo.CreateItem(ctx, workspaceID, label, description, parentID, createdBy)
	if item == nil {
		return nil, domain.ErrNotFound
	}
	return item, nil
}
