package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
)

// GetTree returns the tree metadata for a workspace. The workspace_root
// tree_item is created atomically with the workspace itself
// (CreateWorkspace), so a missing root here means the workspace does
// not exist or has been corrupted — we surface ErrNotFound rather than
// papering over it.
func (s *Store) GetTree(ctx context.Context, wsID string) (*domain.Tree, error) {
	root, err := s.q().GetTreeRoot(ctx, wsID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get tree root: %w", err)
	}
	return &domain.Tree{
		TreeID:      wsID,
		WorkspaceID: wsID,
		Name:        "default",
		CreatedAt:   root.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   root.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *Store) GetTreeByWorkspace(ctx context.Context, wsID string) ([]*domain.Item, error) {
	rows, err := s.q().ListItemsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, fmt.Errorf("list items by workspace: %w", err)
	}
	var items []*domain.Item
	for _, r := range rows {
		items = append(items, toItemFromItemRow(r))
	}
	s.populateChildIDs(ctx, items)
	return items, nil
}

func (s *Store) GetWorkspaceRootItemID(ctx context.Context, wsID string) (string, error) {
	root, err := s.q().GetTreeRoot(ctx, wsID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrNotFound
		}
		return "", err
	}
	return root.ID, nil
}

func (s *Store) FindPaths(ctx context.Context, wsID, sourceItemID, targetItemID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error) {
	items, err := s.GetTreeByWorkspace(ctx, wsID)
	if err != nil || len(items) == 0 {
		return nil, nil, err
	}

	itemByID := make(map[string]*domain.Item, len(items))
	for _, item := range items {
		itemByID[item.ItemID] = item
	}

	if itemByID[sourceItemID] == nil || itemByID[targetItemID] == nil {
		return nil, nil, domain.ErrNotFound
	}

	var paths []domain.TreePath
	curr := sourceItemID
	var sourcePath []string
	for curr != "" && itemByID[curr] != nil {
		sourcePath = append(sourcePath, curr)
		if curr == targetItemID {
			paths = append(paths, domain.TreePath{ItemIDs: sourcePath, HopCount: len(sourcePath) - 1})
			return items, paths, nil
		}
		curr = itemByID[curr].ParentID
	}

	return items, paths, nil
}

func (s *Store) GetSubtree(ctx context.Context, rootItemID string, maxDepth int) ([]*domain.SubtreeItem, error) {
	// ルートアイテム取得
	rootRow, err := s.q().GetItem(ctx, rootItemID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	// 子要素取得
	rows, err := s.q().ListChildItems(ctx, sql.NullString{String: rootItemID, Valid: true})
	if err != nil {
		return nil, err
	}

	var items []*domain.SubtreeItem
	items = append(items, &domain.SubtreeItem{
		Item: *toItemFromGetRow(rootRow),
	})

	for _, r := range rows {
		items = append(items, &domain.SubtreeItem{
			Item:        *toItemFromChildRow(r),
			HasChildren: r.HasChildren,
		})
	}

	plainItems := make([]*domain.Item, 0, len(items))
	for _, item := range items {
		plainItems = append(plainItems, &item.Item)
	}
	s.populateChildIDs(ctx, plainItems)
	return items, nil
}

func (s *Store) populateChildIDs(ctx context.Context, items []*domain.Item) {
	for _, item := range items {
		if item == nil || item.ItemID == "" {
			continue
		}
		rows, err := s.q().ListChildItems(ctx, sql.NullString{String: item.ItemID, Valid: true})
		if err != nil {
			continue
		}
		childIDs := make([]string, 0, len(rows))
		for _, row := range rows {
			childIDs = append(childIDs, row.ID)
		}
		item.ChildIDs = childIDs
	}
}
