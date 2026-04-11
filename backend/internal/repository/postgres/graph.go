package postgres

import (
	"fmt"
	"slices"

	"github.com/synthify/backend/internal/domain"
)

func (s *Store) GetGraph(docID string) ([]*domain.Node, []*domain.Edge, bool) {
	nodes, err := s.listNodesByDocument(docID)
	if err != nil || len(nodes) == 0 {
		return nil, nil, false
	}
	edges, err := s.listEdgesByDocument(docID)
	if err != nil {
		return nil, nil, false
	}
	return nodes, edges, true
}

func (s *Store) FindPaths(docID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool) {
	nodes, edges, ok := s.GetGraph(docID)
	if !ok {
		return nil, nil, nil, false
	}
	if maxDepth <= 0 {
		maxDepth = 4
	}
	if limit <= 0 {
		limit = 3
	}

	nodeByID := make(map[string]*domain.Node, len(nodes))
	for _, node := range nodes {
		nodeByID[node.NodeID] = node
	}
	if nodeByID[sourceNodeID] == nil || nodeByID[targetNodeID] == nil {
		return nil, nil, nil, false
	}
	adj := make(map[string][]string)
	for _, edge := range edges {
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
		adj[edge.TargetNodeID] = append(adj[edge.TargetNodeID], edge.SourceNodeID)
	}
	type item struct {
		nodeID string
		path   []string
	}
	queue := []item{{nodeID: sourceNodeID, path: []string{sourceNodeID}}}
	var paths []domain.GraphPath
	seen := map[string]bool{}

	for len(queue) > 0 && len(paths) < limit {
		cur := queue[0]
		queue = queue[1:]
		if len(cur.path)-1 > maxDepth {
			continue
		}
		if cur.nodeID == targetNodeID {
			key := fmt.Sprint(cur.path)
			if seen[key] {
				continue
			}
			seen[key] = true
			var path domain.GraphPath
			path.NodeIDs = append(path.NodeIDs, cur.path...)
			path.HopCount = len(cur.path) - 1
			path.Evidence.SourceDocumentIDs = []string{docID}
			paths = append(paths, path)
			continue
		}
		for _, next := range adj[cur.nodeID] {
			if slices.Contains(cur.path, next) {
				continue
			}
			nextPath := append(append([]string(nil), cur.path...), next)
			queue = append(queue, item{nodeID: next, path: nextPath})
		}
	}
	return nodes, edges, paths, true
}

func (s *Store) listNodesByDocument(docID string) ([]*domain.Node, error) {
	rows, err := s.db.Query(`
		SELECT node_id, document_id, label, level, category, entity_type, description, summary_html, created_by, created_at
		FROM nodes
		WHERE document_id = $1
		ORDER BY level ASC, created_at ASC
	`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*domain.Node
	for rows.Next() {
		node, err := scanNode(rows)
		if err == nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func (s *Store) listEdgesByDocument(docID string) ([]*domain.Edge, error) {
	rows, err := s.db.Query(`
		SELECT edge_id, document_id, source_node_id, target_node_id, edge_type, description, created_at
		FROM edges
		WHERE document_id = $1
		ORDER BY created_at ASC
	`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []*domain.Edge
	for rows.Next() {
		edge, err := scanEdge(rows)
		if err == nil {
			edges = append(edges, edge)
		}
	}
	return edges, nil
}
