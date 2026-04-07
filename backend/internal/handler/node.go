package handler

import (
	"net/http"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/mock"
)

type NodeHandler struct {
	store *mock.Store
}

func NewNodeHandler(store *mock.Store) *NodeHandler {
	return &NodeHandler{store: store}
}

func (h *NodeHandler) GetGraphEntityDetail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetRef struct {
			WorkspaceID string `json:"workspace_id"`
			Scope       string `json:"scope"`
			ID          string `json:"id"`
			DocumentID  string `json:"document_id"`
		} `json:"target_ref"`
		ResolveAliases bool `json:"resolve_aliases"`
	}
	if err := decodeBody(r, &req); err != nil || req.TargetRef.ID == "" {
		writeError(w, http.StatusBadRequest, "target_ref.id is required")
		return
	}

	node, relatedEdges, ok := h.store.GetNode(req.TargetRef.ID)
	if !ok {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}

	graphNode := domain.GraphNode{
		ID:          node.NodeID,
		Scope:       "document",
		Label:       node.Label,
		Level:       node.Level,
		Category:    node.Category,
		EntityType:  node.EntityType,
		Description: node.Description,
		SummaryHTML: node.SummaryHTML,
	}

	edges := make([]domain.GraphEdge, 0, len(relatedEdges))
	for _, e := range relatedEdges {
		edges = append(edges, domain.GraphEdge{
			ID:     e.EdgeID,
			Source: e.SourceNodeID,
			Target: e.TargetNodeID,
			Type:   e.EdgeType,
			Scope:  "document",
		})
	}

	// モック: 出典チャンクを生成
	chunk := domain.DocumentChunk{
		ChunkID:    "chunk_" + node.NodeID,
		DocumentID: node.DocumentID,
		Heading:    node.Label,
		Text:       node.Description + "（出典チャンクのサンプルテキスト。実際の処理ではドキュメントの該当箇所が入ります。）",
		SourcePage: 3,
	}

	writeJSON(w, map[string]any{
		"detail": map[string]any{
			"ref": map[string]any{
				"workspace_id": req.TargetRef.WorkspaceID,
				"scope":        "document",
				"id":           node.NodeID,
				"document_id":  node.DocumentID,
			},
			"node":          graphNode,
			"related_edges": edges,
			"evidence": map[string]any{
				"source_chunks":      []domain.DocumentChunk{chunk},
				"source_document_ids": []string{node.DocumentID},
			},
			"representative_nodes": []any{},
		},
	})
}

func (h *NodeHandler) RecordNodeView(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID string `json:"workspace_id"`
		NodeID      string `json:"node_id"`
		DocumentID  string `json:"document_id"`
	}
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	h.store.RecordView("user_demo", req.WorkspaceID, req.NodeID, req.DocumentID)
	writeJSON(w, map[string]any{})
}

func (h *NodeHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID  string `json:"workspace_id"`
		DocumentID   string `json:"document_id"`
		Label        string `json:"label"`
		Category     string `json:"category"`
		Level        int    `json:"level"`
		Description  string `json:"description"`
		ParentNodeID string `json:"parent_node_id"`
	}
	if err := decodeBody(r, &req); err != nil || req.DocumentID == "" || req.Label == "" {
		writeError(w, http.StatusBadRequest, "document_id and label are required")
		return
	}
	if req.Category == "" {
		req.Category = "concept"
	}
	node := h.store.CreateNode(req.DocumentID, req.Label, req.Category, req.Description, req.ParentNodeID, req.Level, "user_demo")
	writeJSON(w, map[string]any{
		"node": domain.GraphNode{
			ID:          node.NodeID,
			Scope:       "document",
			Label:       node.Label,
			Level:       node.Level,
			Category:    node.Category,
			Description: node.Description,
		},
	})
}
