package handler

import (
	"context"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// TreeUsecase defines the business operations required for querying workspace taxonomy trees.
type TreeUsecase interface {
	// GetTree retrieves all items comprising the taxonomy tree of the workspace.
	GetTree(ctx context.Context, workspaceID, userID string) ([]*domain.Item, error)
	// GetSubtree retrieves a subtree rooted at the specified item up to maxDepth.
	GetSubtree(ctx context.Context, workspaceID, itemID, userID string, maxDepth int) ([]*domain.SubtreeItem, error)
	// FindPaths discovers relationship paths between a source item and a target item.
	FindPaths(ctx context.Context, workspaceID, sourceItemID, targetItemID, userID string, maxDepth, limit int) ([]*domain.Item, []domain.TreePath, error)
}

// TreeHandler implements Connect-RPC endpoints for workspace taxonomy trees and path traversal.
type TreeHandler struct {
	service TreeUsecase
}

// NewTreeHandler constructs a new TreeHandler backed by the given TreeUsecase.
func NewTreeHandler(svc TreeUsecase) *TreeHandler {
	return &TreeHandler{service: svc}
}

// GetTree returns the entire hierarchical item tree for a workspace.
func (h *TreeHandler) GetTree(ctx context.Context, req *connect.Request[appv1.GetTreeRequest]) (*connect.Response[appv1.GetTreeResponse], error) {
	userID, err := requireUserIDOrShareToken(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.service.GetTree(ctx, req.Msg.GetWorkspaceId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	tree := &appv1.Tree{WorkspaceId: req.Msg.GetWorkspaceId()}
	for _, item := range items {
		tree.Items = append(tree.Items, toProtoItem(item))
	}
	return connect.NewResponse(&appv1.GetTreeResponse{Tree: tree}), nil
}

// GetSubtree returns the subtree rooted at a specific item up to a requested depth.
func (h *TreeHandler) GetSubtree(ctx context.Context, req *connect.Request[appv1.GetSubtreeRequest]) (*connect.Response[appv1.GetSubtreeResponse], error) {
	wsID := req.Msg.GetWorkspaceId()
	userID, err := requireUserIDOrShareToken(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.service.GetSubtree(ctx, wsID, req.Msg.GetItemId(), userID, int(req.Msg.GetMaxDepth()))
	if err != nil {
		return nil, toError(err)
	}
	protoItems := make([]*appv1.SubtreeItem, len(items))
	for i, item := range items {
		protoItems[i] = toProtoSubtreeItem(item)
	}
	return connect.NewResponse(&appv1.GetSubtreeResponse{Items: protoItems}), nil
}

// FindPaths finds relationship paths between source and target items within a workspace.
func (h *TreeHandler) FindPaths(ctx context.Context, req *connect.Request[appv1.FindPathsRequest]) (*connect.Response[appv1.FindPathsResponse], error) {
	userID, err := requireUserIDOrShareToken(ctx)
	if err != nil {
		return nil, err
	}
	items, paths, err := h.service.FindPaths(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetSourceItemId(), req.Msg.GetTargetItemId(), userID, int(req.Msg.GetMaxDepth()), int(req.Msg.GetLimit()))
	if err != nil {
		return nil, toError(err)
	}
	protoTree := &appv1.Tree{WorkspaceId: req.Msg.GetWorkspaceId(), CrossDocument: req.Msg.GetCrossDocument()}
	for _, item := range items {
		protoTree.Items = append(protoTree.Items, toProtoItem(item))
	}
	res := connect.NewResponse(&appv1.FindPathsResponse{Tree: protoTree})
	for _, path := range paths {
		res.Msg.Paths = append(res.Msg.Paths, &appv1.TreePath{ItemIds: path.ItemIDs, HopCount: int32(path.HopCount), EvidenceRef: &appv1.PathEvidenceRef{SourceDocumentIds: path.Evidence.SourceDocumentIDs}})
	}
	return res, nil
}
