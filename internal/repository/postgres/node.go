package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Keyhole-Koro/SynthifyShared/domain"
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
	node := s.CreateStructuredNode(graphID, label, "", 0, "", description, "", createdBy)
	if node == nil || parentNodeID == "" {
		return node
	}
	if s.CreateEdge(graphID, parentNodeID, node.NodeID, "hierarchical", "") == nil {
		return nil
	}
	return node
}

func (s *Store) CreateStructuredNode(graphID, label, category string, level int, entityType, description, summaryHTML, createdBy string) *domain.Node {
	createdAt := nowTime()
	node := &domain.Node{
		NodeID:      newID(),
		GraphID:     graphID,
		Label:       label,
		Category:    category,
		Level:       level,
		EntityType:  entityType,
		Description: description,
		SummaryHTML: summaryHTML,
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

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO nodes (node_id, graph_id, label, category, entity_type, description, summary_html, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, node.NodeID, node.GraphID, node.Label, node.Category, node.EntityType, node.Description, node.SummaryHTML, node.CreatedBy, createdAt); err != nil {
		return nil
	}
	if summaryHTML != "" {
		node.SummaryHTML = summaryHTML
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

func (s *Store) CreateEdge(graphID, sourceNodeID, targetNodeID, edgeType, description string) *domain.Edge {
	createdAt := nowTime()
	edge := &domain.Edge{
		EdgeID:       newID(),
		GraphID:      graphID,
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		EdgeType:     edgeType,
		Description:  description,
		CreatedAt:    createdAt.Format(time.RFC3339),
	}
	if _, err := s.db.ExecContext(context.Background(), `
		INSERT INTO edges (edge_id, graph_id, source_node_id, target_node_id, edge_type, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, edge.EdgeID, edge.GraphID, edge.SourceNodeID, edge.TargetNodeID, edge.EdgeType, edge.Description, createdAt); err != nil {
		return nil
	}
	_ = s.q().UpdateGraphTimestamp(context.Background(), sqlcgen.UpdateGraphTimestampParams{
		GraphID:   graphID,
		UpdatedAt: nowTime(),
	})
	return edge
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

func (s *Store) UpsertEdgeSource(edgeID, documentID, chunkID, sourceText string, confidence float64) error {
	return s.q().UpsertEdgeSource(context.Background(), sqlcgen.UpsertEdgeSourceParams{
		EdgeID:     edgeID,
		DocumentID: documentID,
		ChunkID:    chunkID,
		SourceText: sourceText,
		Confidence: sql.NullFloat64{Float64: confidence, Valid: confidence > 0},
	})
}

func (s *Store) UpdateNodeSummaryHTML(nodeID, summaryHTML string) bool {
	result, err := s.db.ExecContext(context.Background(), `
		UPDATE nodes SET summary_html = $2 WHERE node_id = $1
	`, nodeID, summaryHTML)
	if err != nil {
		return false
	}
	affected, _ := result.RowsAffected()
	return affected > 0
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
