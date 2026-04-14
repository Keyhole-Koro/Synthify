package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	graphv1 "github.com/synthify/backend/gen/synthify/graph/v1"
	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository"
	"github.com/synthify/backend/internal/service"
)

type NodeHandler struct {
	service    *service.NodeService
	workspaces repository.WorkspaceRepository
	documents  repository.DocumentRepository
	nodes      repository.NodeRepository
}

func NewNodeHandler(
	svc *service.NodeService,
	workspaceRepo repository.WorkspaceRepository,
	documentRepo repository.DocumentRepository,
	nodeRepo repository.NodeRepository,
) *NodeHandler {
	return &NodeHandler{
		service:    svc,
		workspaces: workspaceRepo,
		documents:  documentRepo,
		nodes:      nodeRepo,
	}
}

func (h *NodeHandler) GetGraphEntityDetail(ctx context.Context, req *connect.Request[graphv1.GetGraphEntityDetailRequest]) (*connect.Response[graphv1.GetGraphEntityDetailResponse], error) {
	if req.Msg.GetTargetRef() == nil || req.Msg.GetTargetRef().GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_ref.id is required"))
	}
	if err := authorizeNode(ctx, h.workspaces, h.documents, h.nodes, req.Msg.GetTargetRef().GetId(), req.Msg.GetTargetRef().GetWorkspaceId()); err != nil {
		return nil, err
	}

	node, relatedEdges, err := h.service.GetGraphEntityDetail(req.Msg.GetTargetRef().GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	chunk := &graphv1.DocumentChunk{
		ChunkId:        "chunk_" + node.NodeID,
		DocumentId:     node.DocumentID,
		Text:           node.Description + "（出典チャンクのサンプルテキスト。実際の処理ではドキュメントの該当箇所が入ります。）",
		SourceFilename: "sample.txt",
		SourcePage:     3,
	}

	detail := &graphv1.GraphEntityDetail{
		Ref: &graphv1.EntityRef{
			WorkspaceId: req.Msg.GetTargetRef().GetWorkspaceId(),
			Scope:       graphv1.GraphProjectionScope_GRAPH_PROJECTION_SCOPE_DOCUMENT,
			Id:          node.NodeID,
			DocumentId:  node.DocumentID,
		},
		Node: toProtoNode(node),
		Evidence: &graphv1.GraphEntityEvidence{
			SourceChunks:      []*graphv1.DocumentChunk{chunk},
			SourceDocumentIds: []string{node.DocumentID},
		},
	}
	for _, edge := range relatedEdges {
		detail.RelatedEdges = append(detail.RelatedEdges, toProtoEdge(edge))
	}
	return connect.NewResponse(&graphv1.GetGraphEntityDetailResponse{Detail: detail}), nil
}

func (h *NodeHandler) RecordNodeView(ctx context.Context, req *connect.Request[graphv1.RecordNodeViewRequest]) (*connect.Response[graphv1.RecordNodeViewResponse], error) {
	if err := authorizeDocument(ctx, h.workspaces, h.documents, req.Msg.GetDocumentId(), req.Msg.GetWorkspaceId()); err != nil {
		return nil, err
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	h.service.RecordNodeView(user.ID, req.Msg.GetWorkspaceId(), req.Msg.GetNodeId(), req.Msg.GetDocumentId())
	return connect.NewResponse(&graphv1.RecordNodeViewResponse{}), nil
}

func (h *NodeHandler) CreateNode(ctx context.Context, req *connect.Request[graphv1.CreateNodeRequest]) (*connect.Response[graphv1.CreateNodeResponse], error) {
	if req.Msg.GetDocumentId() == "" || req.Msg.GetLabel() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id and label are required"))
	}
	if err := authorizeDocument(ctx, h.workspaces, h.documents, req.Msg.GetDocumentId(), req.Msg.GetWorkspaceId()); err != nil {
		return nil, err
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	category := req.Msg.GetCategory()
	if category == "" {
		category = "concept"
	}
	node := h.service.CreateNode(user.ID, req.Msg.GetDocumentId(), req.Msg.GetLabel(), category, req.Msg.GetDescription(), req.Msg.GetParentNodeId(), int(req.Msg.GetLevel()))
	return connect.NewResponse(&graphv1.CreateNodeResponse{Node: toProtoNode(node)}), nil
}

func (h *NodeHandler) GetUserNodeActivity(ctx context.Context, req *connect.Request[graphv1.GetUserNodeActivityRequest]) (*connect.Response[graphv1.GetUserNodeActivityResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	if err := authorizeWorkspace(ctx, h.workspaces, req.Msg.GetWorkspaceId()); err != nil {
		return nil, err
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	activity := h.service.GetUserNodeActivity(req.Msg.GetWorkspaceId(), user.ID, req.Msg.GetDocumentId(), int(req.Msg.GetLimit()))
	return connect.NewResponse(&graphv1.GetUserNodeActivityResponse{
		Activity: &graphv1.UserNodeActivity{
			UserId:       activity.UserID,
			DisplayName:  activity.DisplayName,
			ViewedNodes:  toProtoViewedNodes(activity.ViewedNodes),
			CreatedNodes: toProtoCreatedNodes(activity.CreatedNodes),
		},
	}), nil
}

func (h *NodeHandler) ApproveAlias(ctx context.Context, req *connect.Request[graphv1.ApproveAliasRequest]) (*connect.Response[graphv1.ApproveAliasResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetCanonicalNodeId() == "" || req.Msg.GetAliasNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id, canonical_node_id, and alias_node_id are required"))
	}
	if err := authorizeWorkspace(ctx, h.workspaces, req.Msg.GetWorkspaceId()); err != nil {
		return nil, err
	}
	if err := h.service.ApproveAlias(req.Msg.GetWorkspaceId(), req.Msg.GetCanonicalNodeId(), req.Msg.GetAliasNodeId()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&graphv1.ApproveAliasResponse{
		CanonicalNodeId: req.Msg.GetCanonicalNodeId(),
		AliasNodeId:     req.Msg.GetAliasNodeId(),
		MergeStatus:     "approved",
	}), nil
}

func (h *NodeHandler) RejectAlias(ctx context.Context, req *connect.Request[graphv1.RejectAliasRequest]) (*connect.Response[graphv1.RejectAliasResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetCanonicalNodeId() == "" || req.Msg.GetAliasNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id, canonical_node_id, and alias_node_id are required"))
	}
	if err := authorizeWorkspace(ctx, h.workspaces, req.Msg.GetWorkspaceId()); err != nil {
		return nil, err
	}
	if err := h.service.RejectAlias(req.Msg.GetWorkspaceId(), req.Msg.GetCanonicalNodeId(), req.Msg.GetAliasNodeId()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&graphv1.RejectAliasResponse{
		CanonicalNodeId: req.Msg.GetCanonicalNodeId(),
		AliasNodeId:     req.Msg.GetAliasNodeId(),
		MergeStatus:     "rejected",
	}), nil
}

func toProtoNode(node *domain.Node) *graphv1.Node {
	return &graphv1.Node{
		Id:          node.NodeID,
		DocumentId:  node.DocumentID,
		Label:       node.Label,
		Level:       int32(node.Level),
		Category:    nodeCategoryToProto(node.Category),
		EntityType:  nodeEntityTypeToProto(node.EntityType),
		Description: node.Description,
		SummaryHtml: node.SummaryHTML,
		CreatedAt:   node.CreatedAt,
		Scope:       graphv1.GraphProjectionScope_GRAPH_PROJECTION_SCOPE_DOCUMENT,
	}
}

func toProtoEdge(edge *domain.Edge) *graphv1.Edge {
	return &graphv1.Edge{
		Id:          edge.EdgeID,
		Source:      edge.SourceNodeID,
		Target:      edge.TargetNodeID,
		Type:        edge.EdgeType,
		Scope:       graphv1.GraphProjectionScope_GRAPH_PROJECTION_SCOPE_DOCUMENT,
		Description: edge.Description,
		CreatedAt:   edge.CreatedAt,
	}
}

func toProtoViewedNodes(entries []domain.ViewedNodeEntry) []*graphv1.ViewedNodeEntry {
	out := make([]*graphv1.ViewedNodeEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, &graphv1.ViewedNodeEntry{
			NodeId:       entry.NodeID,
			DocumentId:   entry.DocumentID,
			Label:        entry.Label,
			LastViewedAt: entry.LastViewedAt,
			ViewCount:    entry.ViewCount,
		})
	}
	return out
}

func toProtoCreatedNodes(entries []domain.CreatedNodeEntry) []*graphv1.CreatedNodeEntry {
	out := make([]*graphv1.CreatedNodeEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, &graphv1.CreatedNodeEntry{
			NodeId:     entry.NodeID,
			DocumentId: entry.DocumentID,
			Label:      entry.Label,
			CreatedAt:  entry.CreatedAt,
		})
	}
	return out
}
