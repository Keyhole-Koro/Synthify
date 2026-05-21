package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/service"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type TreeHandler struct {
	service service.TreeUsecase
}

func NewTreeHandler(svc service.TreeUsecase) *TreeHandler {
	return &TreeHandler{service: svc}
}

func (h *TreeHandler) GetTree(ctx context.Context, req *connect.Request[appv1.GetTreeRequest]) (*connect.Response[appv1.GetTreeResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
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

func (h *TreeHandler) GetSubtree(ctx context.Context, req *connect.Request[appv1.GetSubtreeRequest]) (*connect.Response[appv1.GetSubtreeResponse], error) {
	wsID := req.Msg.GetWorkspaceId()
	if wsID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	if req.Msg.GetItemId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("item_id is required"))
	}
	userID, err := requireUserID(ctx)
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

func (h *TreeHandler) FindPaths(ctx context.Context, req *connect.Request[appv1.FindPathsRequest]) (*connect.Response[appv1.FindPathsResponse], error) {
	if req.Msg.GetSourceItemId() == "" || req.Msg.GetTargetItemId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_item_id and target_item_id are required"))
	}
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}

	items, paths, err := h.service.FindPaths(
		ctx,
		req.Msg.GetWorkspaceId(),
		req.Msg.GetSourceItemId(),
		req.Msg.GetTargetItemId(),
		userID,
		int(req.Msg.GetMaxDepth()),
		int(req.Msg.GetLimit()),
	)
	if err != nil {
		return nil, toError(err)
	}

	protoTree := &appv1.Tree{
		WorkspaceId:   req.Msg.GetWorkspaceId(),
		CrossDocument: req.Msg.GetCrossDocument(),
	}
	for _, item := range items {
		protoTree.Items = append(protoTree.Items, toProtoItem(item))
	}

	res := connect.NewResponse(&appv1.FindPathsResponse{Tree: protoTree})
	for _, path := range paths {
		res.Msg.Paths = append(res.Msg.Paths, &appv1.TreePath{
			ItemIds:  path.ItemIDs,
			HopCount: int32(path.HopCount),
			EvidenceRef: &appv1.PathEvidenceRef{
				SourceDocumentIds: path.Evidence.SourceDocumentIDs,
			},
		})
	}
	return res, nil
}
