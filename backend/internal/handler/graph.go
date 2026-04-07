package handler

import (
	"net/http"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/mock"
)

type GraphHandler struct {
	store *mock.Store
}

func NewGraphHandler(store *mock.Store) *GraphHandler {
	return &GraphHandler{store: store}
}

func (h *GraphHandler) GetGraph(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID   string   `json:"workspace_id"`
		DocumentID    string   `json:"document_id"`
		LevelFilters  []int    `json:"level_filters"`
		CategoryFilters []string `json:"category_filters"`
		Limit         int      `json:"limit"`
	}
	if err := decodeBody(r, &req); err != nil || req.DocumentID == "" {
		writeError(w, http.StatusBadRequest, "document_id is required")
		return
	}

	nodes, edges, ok := h.store.GetGraph(req.DocumentID)
	if !ok {
		writeError(w, http.StatusNotFound, "graph not found for document")
		return
	}

	// Apply level filter
	levelSet := make(map[int]bool)
	for _, l := range req.LevelFilters {
		levelSet[l] = true
	}

	// Apply category filter
	catSet := make(map[string]bool)
	for _, c := range req.CategoryFilters {
		catSet[c] = true
	}

	graphNodes := make([]domain.GraphNode, 0, len(nodes))
	nodeIDs := make(map[string]bool)
	for _, n := range nodes {
		if len(levelSet) > 0 && !levelSet[n.Level] {
			continue
		}
		if len(catSet) > 0 && !catSet[n.Category] {
			continue
		}
		graphNodes = append(graphNodes, domain.GraphNode{
			ID:          n.NodeID,
			Scope:       "document",
			Label:       n.Label,
			Level:       n.Level,
			Category:    n.Category,
			EntityType:  n.EntityType,
			Description: n.Description,
			SummaryHTML: n.SummaryHTML,
		})
		nodeIDs[n.NodeID] = true
	}

	graphEdges := make([]domain.GraphEdge, 0, len(edges))
	for _, e := range edges {
		if !nodeIDs[e.SourceNodeID] || !nodeIDs[e.TargetNodeID] {
			continue
		}
		graphEdges = append(graphEdges, domain.GraphEdge{
			ID:     e.EdgeID,
			Source: e.SourceNodeID,
			Target: e.TargetNodeID,
			Type:   e.EdgeType,
			Scope:  "document",
		})
	}

	writeJSON(w, map[string]any{
		"graph": map[string]any{
			"nodes": graphNodes,
			"edges": graphEdges,
		},
	})
}
