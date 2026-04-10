package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	graphv1 "github.com/synthify/backend/gen/synthify/graph/v1"
	"github.com/synthify/backend/internal/service"
)

type GraphHandler struct {
	service *service.GraphService
}

func NewGraphHandler(svc *service.GraphService) *GraphHandler {
	return &GraphHandler{service: svc}
}

func (h *GraphHandler) GetGraph(_ context.Context, req *connect.Request[graphv1.GetGraphRequest]) (*connect.Response[graphv1.GetGraphResponse], error) {
	if req.Msg.GetDocumentId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("document_id is required"))
	}
	nodes, edges, err := h.service.GetGraph(req.Msg.GetDocumentId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	levelSet := map[int32]bool{}
	for _, level := range req.Msg.GetLevelFilters() {
		levelSet[level] = true
	}
	categorySet := map[graphv1.NodeCategory]bool{}
	for _, category := range req.Msg.GetCategoryFilters() {
		categorySet[category] = true
	}

	graph := &graphv1.Graph{
		WorkspaceId: req.Msg.GetWorkspaceId(),
		DocumentId:  req.Msg.GetDocumentId(),
	}
	nodeIDs := map[string]bool{}
	for _, node := range nodes {
		if len(levelSet) > 0 && !levelSet[int32(node.Level)] {
			continue
		}
		if len(categorySet) > 0 && !categorySet[nodeCategoryToProto(node.Category)] {
			continue
		}
		protoNode := toProtoNode(node)
		graph.Nodes = append(graph.Nodes, protoNode)
		nodeIDs[protoNode.GetId()] = true
	}
	for _, edge := range edges {
		if !nodeIDs[edge.SourceNodeID] || !nodeIDs[edge.TargetNodeID] {
			continue
		}
		graph.Edges = append(graph.Edges, toProtoEdge(edge))
	}
	return connect.NewResponse(&graphv1.GetGraphResponse{Graph: graph}), nil
}

func (h *GraphHandler) FindPaths(_ context.Context, req *connect.Request[graphv1.FindPathsRequest]) (*connect.Response[graphv1.FindPathsResponse], error) {
	if req.Msg.GetSourceNodeId() == "" || req.Msg.GetTargetNodeId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_node_id and target_node_id are required"))
	}
	docID := ""
	if ids := req.Msg.GetDocumentIds(); len(ids) > 0 {
		docID = ids[0]
	}
	if docID == "" {
		docID = "doc_sales"
	}

	nodes, edges, paths, err := h.service.FindPaths(docID, req.Msg.GetSourceNodeId(), req.Msg.GetTargetNodeId(), int(req.Msg.GetMaxDepth()), int(req.Msg.GetLimit()))
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	graph := &graphv1.Graph{
		WorkspaceId:   req.Msg.GetWorkspaceId(),
		DocumentId:    docID,
		CrossDocument: req.Msg.GetCrossDocument(),
	}
	for _, node := range nodes {
		graph.Nodes = append(graph.Nodes, toProtoNode(node))
	}
	for _, edge := range edges {
		graph.Edges = append(graph.Edges, toProtoEdge(edge))
	}

	res := connect.NewResponse(&graphv1.FindPathsResponse{Graph: graph})
	for _, path := range paths {
		res.Msg.Paths = append(res.Msg.Paths, &graphv1.GraphPath{
			NodeIds:  path.NodeIDs,
			HopCount: int32(path.HopCount),
			EvidenceRef: &graphv1.PathEvidenceRef{
				SourceDocumentIds: path.Evidence.SourceDocumentIDs,
			},
		})
	}
	return res, nil
}
