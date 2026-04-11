package postgres

import (
	"context"
	"time"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

func (s *Store) GetNode(nodeID string) (*domain.Node, []*domain.Edge, bool) {
	ctx := context.Background()
	row, err := s.q().GetNode(ctx, nodeID)
	if err != nil {
		return nil, nil, false
	}
	edgeRows, err := s.q().ListNodeEdges(ctx, nodeID)
	if err != nil {
		return nil, nil, false
	}

	var edges []*domain.Edge
	for _, edgeRow := range edgeRows {
		edges = append(edges, toEdge(edgeRow))
	}
	return toNode(row), edges, true
}

func (s *Store) CreateNode(docID, label, category, description, parentNodeID string, level int, createdBy string) *domain.Node {
	createdAt := nowTime()
	node := &domain.Node{
		NodeID:      newID("nd"),
		DocumentID:  docID,
		Label:       label,
		Level:       level,
		Category:    category,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   createdAt.Format(time.RFC3339),
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil
	}
	defer tx.Rollback()
	qtx := s.q().WithTx(tx)
	ctx := context.Background()

	if err := qtx.CreateNode(ctx, sqlcgen.CreateNodeParams{
		NodeID:      node.NodeID,
		DocumentID:  node.DocumentID,
		Label:       node.Label,
		Level:       int32(node.Level),
		Category:    node.Category,
		EntityType:  node.EntityType,
		Description: node.Description,
		SummaryHtml: node.SummaryHTML,
		CreatedBy:   node.CreatedBy,
		CreatedAt:   createdAt,
	}); err != nil {
		return nil
	}
	if parentNodeID != "" {
		if err := qtx.CreateEdge(ctx, sqlcgen.CreateEdgeParams{
			EdgeID:       newID("ed"),
			DocumentID:   docID,
			SourceNodeID: parentNodeID,
			TargetNodeID: node.NodeID,
			EdgeType:     "hierarchical",
			Description:  "",
			CreatedAt:    nowTime(),
		}); err != nil {
			return nil
		}
	}
	_ = qtx.UpdateDocumentTimestamp(ctx, sqlcgen.UpdateDocumentTimestampParams{
		DocumentID: docID,
		UpdatedAt:  nowTime(),
	})
	if err := tx.Commit(); err != nil {
		return nil
	}
	return node
}

func (s *Store) RecordView(userID, wsID, nodeID, docID string) {
	_ = s.q().InsertNodeView(context.Background(), sqlcgen.InsertNodeViewParams{
		WorkspaceID: wsID,
		UserID:      userID,
		NodeID:      nodeID,
		DocumentID:  docID,
		ViewedAt:    nowTime(),
	})
}

func (s *Store) GetUserNodeActivity(wsID, userID, documentID string, limit int) domain.UserNodeActivity {
	if limit <= 0 {
		limit = 50
	}
	ctx := context.Background()
	activity := domain.UserNodeActivity{
		UserID:      userID,
		DisplayName: userID,
	}
	if email, err := s.q().GetWorkspaceMemberEmail(ctx, sqlcgen.GetWorkspaceMemberEmailParams{
		WorkspaceID: wsID,
		UserID:      userID,
	}); err == nil {
		activity.DisplayName = email
	}

	viewRows, err := s.q().ListViewedNodes(ctx, sqlcgen.ListViewedNodesParams{
		WorkspaceID:      wsID,
		UserID:           userID,
		DocumentIDFilter: documentID,
		RowLimit:         int32(limit),
	})
	if err == nil {
		for _, row := range viewRows {
			activity.ViewedNodes = append(activity.ViewedNodes, domain.ViewedNodeEntry{
				NodeID:       row.NodeID,
				DocumentID:   row.DocumentID,
				Label:        row.Label,
				LastViewedAt: row.LastViewedAt.UTC().Format(time.RFC3339),
				ViewCount:    row.ViewCount,
			})
		}
	}

	createdRows, err := s.q().ListCreatedNodes(ctx, sqlcgen.ListCreatedNodesParams{
		CreatedBy:        userID,
		DocumentIDFilter: documentID,
		RowLimit:         int32(limit),
	})
	if err == nil {
		for _, row := range createdRows {
			activity.CreatedNodes = append(activity.CreatedNodes, domain.CreatedNodeEntry{
				NodeID:     row.NodeID,
				DocumentID: row.DocumentID,
				Label:      row.Label,
				CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
	}
	return activity
}

func (s *Store) ApproveAlias(wsID, canonicalNodeID, aliasNodeID string) bool {
	return s.q().UpsertApprovedAlias(context.Background(), sqlcgen.UpsertApprovedAliasParams{
		WorkspaceID:     wsID,
		CanonicalNodeID: canonicalNodeID,
		AliasNodeID:     aliasNodeID,
		UpdatedAt:       nowTime(),
	}) == nil
}

func (s *Store) RejectAlias(wsID, canonicalNodeID, aliasNodeID string) bool {
	return s.q().UpsertRejectedAlias(context.Background(), sqlcgen.UpsertRejectedAliasParams{
		WorkspaceID:     wsID,
		CanonicalNodeID: canonicalNodeID,
		AliasNodeID:     aliasNodeID,
		UpdatedAt:       nowTime(),
	}) == nil
}
