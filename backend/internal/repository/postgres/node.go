package postgres

import (
	"fmt"
	"time"

	"github.com/synthify/backend/internal/domain"
)

func (s *Store) GetNode(nodeID string) (*domain.Node, []*domain.Edge, bool) {
	row := s.db.QueryRow(`
		SELECT node_id, document_id, label, level, category, entity_type, description, summary_html, created_by, created_at
		FROM nodes
		WHERE node_id = $1
	`, nodeID)
	node, err := scanNode(row)
	if err != nil {
		return nil, nil, false
	}
	rows, err := s.db.Query(`
		SELECT edge_id, document_id, source_node_id, target_node_id, edge_type, description, created_at
		FROM edges
		WHERE source_node_id = $1 OR target_node_id = $1
		ORDER BY created_at ASC
	`, nodeID)
	if err != nil {
		return nil, nil, false
	}
	defer rows.Close()

	var edges []*domain.Edge
	for rows.Next() {
		edge, err := scanEdge(rows)
		if err == nil {
			edges = append(edges, edge)
		}
	}
	return node, edges, true
}

func (s *Store) CreateNode(docID, label, category, description, parentNodeID string, level int, createdBy string) *domain.Node {
	node := &domain.Node{
		NodeID:      newID("nd"),
		DocumentID:  docID,
		Label:       label,
		Level:       level,
		Category:    category,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   now(),
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO nodes (node_id, document_id, label, level, category, entity_type, description, summary_html, created_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, node.NodeID, node.DocumentID, node.Label, node.Level, node.Category, node.EntityType, node.Description, node.SummaryHTML, node.CreatedBy, node.CreatedAt)
	if err != nil {
		return nil
	}
	if parentNodeID != "" {
		_, err = tx.Exec(`
			INSERT INTO edges (edge_id, document_id, source_node_id, target_node_id, edge_type, description, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
		`, newID("ed"), docID, parentNodeID, node.NodeID, "hierarchical", "", now())
		if err != nil {
			return nil
		}
	}
	_, _ = tx.Exec(`UPDATE documents SET updated_at = $2 WHERE document_id = $1`, docID, now())
	if err := tx.Commit(); err != nil {
		return nil
	}
	return node
}

func (s *Store) RecordView(userID, wsID, nodeID, docID string) {
	_, _ = s.db.Exec(`
		INSERT INTO node_views (workspace_id, user_id, node_id, document_id, viewed_at)
		VALUES ($1,$2,$3,$4,$5)
	`, wsID, userID, nodeID, docID, now())
}

func (s *Store) GetUserNodeActivity(wsID, userID, documentID string, limit int) domain.UserNodeActivity {
	if limit <= 0 {
		limit = 50
	}
	activity := domain.UserNodeActivity{
		UserID:      userID,
		DisplayName: userID,
	}
	_ = s.db.QueryRow(`SELECT email FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`, wsID, userID).Scan(&activity.DisplayName)

	viewQuery := `
		SELECT nv.node_id, nv.document_id, COALESCE(n.label, nv.node_id) AS label, MAX(nv.viewed_at) AS last_viewed_at, COUNT(*) AS view_count
		FROM node_views nv
		LEFT JOIN nodes n ON n.node_id = nv.node_id
		WHERE nv.workspace_id = $1 AND nv.user_id = $2
	`
	args := []any{wsID, userID}
	if documentID != "" {
		viewQuery += ` AND nv.document_id = $3`
		args = append(args, documentID)
	}
	viewQuery += ` GROUP BY nv.node_id, nv.document_id, label ORDER BY last_viewed_at DESC LIMIT `
	viewQuery += fmt.Sprintf("%d", limit)
	rows, err := s.db.Query(viewQuery, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var entry domain.ViewedNodeEntry
			var viewedAt time.Time
			if err := rows.Scan(&entry.NodeID, &entry.DocumentID, &entry.Label, &viewedAt, &entry.ViewCount); err == nil {
				entry.LastViewedAt = viewedAt.UTC().Format(time.RFC3339)
				activity.ViewedNodes = append(activity.ViewedNodes, entry)
			}
		}
	}

	createQuery := `
		SELECT node_id, document_id, label, created_at
		FROM nodes
		WHERE created_by = $1
	`
	args = []any{userID}
	if documentID != "" {
		createQuery += ` AND document_id = $2`
		args = append(args, documentID)
	}
	createQuery += ` ORDER BY created_at DESC LIMIT `
	createQuery += fmt.Sprintf("%d", limit)
	rows, err = s.db.Query(createQuery, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var entry domain.CreatedNodeEntry
			var createdAt time.Time
			if err := rows.Scan(&entry.NodeID, &entry.DocumentID, &entry.Label, &createdAt); err == nil {
				entry.CreatedAt = createdAt.UTC().Format(time.RFC3339)
				activity.CreatedNodes = append(activity.CreatedNodes, entry)
			}
		}
	}
	return activity
}

func (s *Store) ApproveAlias(wsID, canonicalNodeID, aliasNodeID string) bool {
	_, err := s.db.Exec(`
		INSERT INTO node_aliases (workspace_id, canonical_node_id, alias_node_id, status, updated_at)
		VALUES ($1,$2,$3,'approved',$4)
		ON CONFLICT (workspace_id, canonical_node_id, alias_node_id)
		DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
	`, wsID, canonicalNodeID, aliasNodeID, now())
	return err == nil
}

func (s *Store) RejectAlias(wsID, canonicalNodeID, aliasNodeID string) bool {
	_, err := s.db.Exec(`
		INSERT INTO node_aliases (workspace_id, canonical_node_id, alias_node_id, status, updated_at)
		VALUES ($1,$2,$3,'rejected',$4)
		ON CONFLICT (workspace_id, canonical_node_id, alias_node_id)
		DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at
	`, wsID, canonicalNodeID, aliasNodeID, now())
	return err == nil
}
