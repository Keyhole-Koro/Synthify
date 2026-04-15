package postgres

import (
	"context"
	"database/sql"
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

func (s *Store) CreateNode(graphID, label, description, parentNodeID, createdBy string) *domain.Node {
	createdAt := nowTime()
	node := &domain.Node{
		NodeID:      newID(),
		GraphID:     graphID,
		Label:       label,
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
		GraphID:     node.GraphID,
		Label:       node.Label,
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
			EdgeID:       newID(),
			GraphID:      graphID,
			SourceNodeID: parentNodeID,
			TargetNodeID: node.NodeID,
			EdgeType:     "hierarchical",
			Description:  "",
			CreatedAt:    nowTime(),
		}); err != nil {
			return nil
		}
	}
	_ = qtx.UpdateGraphTimestamp(ctx, sqlcgen.UpdateGraphTimestampParams{
		GraphID:   graphID,
		UpdatedAt: nowTime(),
	})
	if err := tx.Commit(); err != nil {
		return nil
	}
	return node
}

func (s *Store) UpsertNodeSource(nodeID, documentID, chunkID, sourceText string, confidence float64) error {
	return s.q().UpsertNodeSource(context.Background(), sqlcgen.UpsertNodeSourceParams{
		NodeID:     nodeID,
		DocumentID: documentID,
		ChunkID:    chunkID,
		SourceText: sourceText,
		Confidence: sql.NullFloat64{Float64: confidence, Valid: confidence > 0},
	})
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
