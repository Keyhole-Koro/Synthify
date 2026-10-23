package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
)

// GetTree returns the tree metadata for a workspace. The workspace itself is
// the tree root (there is no workspace_root tree_item), so existence and
// timestamps come from the workspace row.
func (s *Store) GetTree(ctx context.Context, wsID string) (*domain.Tree, error) {
	ws, err := s.q().GetWorkspace(ctx, wsID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get tree: %w", err)
	}
	return &domain.Tree{
		TreeID:      wsID,
		WorkspaceID: wsID,
		Name:        "default",
		CreatedAt:   ws.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   ws.CreatedAt.UTC().Format(time.RFC3339),
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
	if err := s.populateChildIDs(ctx, items); err != nil {
		return nil, err
	}
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
	if err := s.populateChildIDs(ctx, plainItems); err != nil {
		return nil, err
	}
	return items, nil
}

// populateChildIDs fills in ChildIDs for every item, using one query per
// distinct workspace rather than one per item. See the
// ListChildIDsByWorkspace query for why: the per-item version was 1+N queries
// that each dragged along the node bodies it did not need.
//
// Errors are returned rather than swallowed. The per-item version skipped a
// failed item and left its ChildIDs empty, which is survivable; one failure
// here would silently flatten the entire tree, which is not.
func (s *Store) populateChildIDs(ctx context.Context, items []*domain.Item) error {
	itemsByID := make(map[string]*domain.Item, len(items))
	workspaceIDs := make([]string, 0, 1)
	for _, item := range items {
		if item == nil || item.ItemID == "" {
			continue
		}
		itemsByID[item.ItemID] = item
		// Every item starts with an empty (non-nil) slice, matching what the
		// per-item version produced for a childless node.
		item.ChildIDs = []string{}
		if item.WorkspaceID != "" && !slices.Contains(workspaceIDs, item.WorkspaceID) {
			workspaceIDs = append(workspaceIDs, item.WorkspaceID)
		}
	}
	if len(itemsByID) == 0 {
		return nil
	}

	for _, wsID := range workspaceIDs {
		rows, err := s.q().ListChildIDsByWorkspace(ctx, wsID)
		if err != nil {
			return fmt.Errorf("list child ids by workspace: %w", err)
		}
		for _, row := range rows {
			// Edges whose parent is not among `items` are skipped: a subtree
			// listing holds only part of the workspace.
			if parent, ok := itemsByID[row.ParentID.String]; ok {
				parent.ChildIDs = append(parent.ChildIDs, row.ID)
			}
		}
	}
	return nil
}
