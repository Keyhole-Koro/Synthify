package postgres

import (
	"context"
	"fmt"
	"slices"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

func (s *Store) GetOrCreateGraph(wsID string) (*domain.Graph, error) {
	createdAt := nowTime()
	row, err := s.q().GetOrCreateGraph(context.Background(), sqlcgen.GetOrCreateGraphParams{
		GraphID:     newID(),
		WorkspaceID: wsID,
		Name:        "default",
		CreatedAt:   createdAt,
	})
	if err != nil {
		return nil, err
	}
	return toGraph(row), nil
}

func (s *Store) GetGraphByWorkspace(wsID string) ([]*domain.Node, []*domain.Edge, bool) {
	ctx := context.Background()
	graphRow, err := s.q().GetGraphByWorkspace(ctx, wsID)
	if err != nil {
		return nil, nil, false
	}
	nodes, err := s.listNodesByGraph(graphRow.GraphID)
	if err != nil {
		return nil, nil, false
	}
	edges, err := s.listEdgesByGraph(graphRow.GraphID)
	if err != nil {
		return nil, nil, false
	}
	return nodes, edges, true
}

func (s *Store) FindPaths(graphID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool) {
	nodes, err := s.listNodesByGraph(graphID)
	if err != nil || len(nodes) == 0 {
		return nil, nil, nil, false
	}
	edges, err := s.listEdgesByGraph(graphID)
	if err != nil {
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

func (s *Store) listNodesByGraph(graphID string) ([]*domain.Node, error) {
	rows, err := s.q().ListNodesByGraph(context.Background(), graphID)
	if err != nil {
		return nil, err
	}
	var nodes []*domain.Node
	for _, row := range rows {
		nodes = append(nodes, toNodeFromListRow(row))
	}
	return nodes, nil
}

func (s *Store) listEdgesByGraph(graphID string) ([]*domain.Edge, error) {
	rows, err := s.q().ListEdgesByGraph(context.Background(), graphID)
	if err != nil {
		return nil, err
	}
	var edges []*domain.Edge
	for _, row := range rows {
		edges = append(edges, toEdge(row))
	}
	return edges, nil
}
