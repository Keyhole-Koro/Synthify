package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
)

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
