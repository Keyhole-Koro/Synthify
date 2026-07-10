package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/service"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type ItemHandler struct {
	service service.ItemUsecase
}

func NewItemHandler(svc service.ItemUsecase) *ItemHandler {
	return &ItemHandler{service: svc}
}

func (h *ItemHandler) GetTreeEntityDetail(ctx context.Context, req *connect.Request[appv1.GetTreeEntityDetailRequest]) (*connect.Response[appv1.GetTreeEntityDetailResponse], error) {
	if req.Msg.GetTargetRef() == nil || req.Msg.GetTargetRef().GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_ref.id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := h.service.GetItem(ctx, req.Msg.GetTargetRef().GetId(), req.Msg.GetTargetRef().GetWorkspaceId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	evidence, err := h.service.GetItemEvidence(ctx, item.ItemID, req.Msg.GetTargetRef().GetWorkspaceId(), userID)
	if err != nil {
		return nil, toError(err)
	}

	detail := &appv1.TreeEntityDetail{
		Ref: &appv1.EntityRef{
			WorkspaceId: req.Msg.GetTargetRef().GetWorkspaceId(),
			Scope:       item.Scope,
			Id:          item.ItemID,
		},
		Item:     toProtoItem(item),
		Evidence: toProtoItemEvidence(evidence),
	}
	return connect.NewResponse(&appv1.GetTreeEntityDetailResponse{Detail: detail}), nil
}

func (h *ItemHandler) CreateItem(ctx context.Context, req *connect.Request[appv1.CreateItemRequest]) (*connect.Response[appv1.CreateItemResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetLabel() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and label are required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := h.service.CreateItem(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetLabel(), req.Msg.GetDescription(), req.Msg.GetParentId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.CreateItemResponse{Item: toProtoItem(item)}), nil
}

func (h *ItemHandler) ApproveAlias(ctx context.Context, req *connect.Request[appv1.ApproveAliasRequest]) (*connect.Response[appv1.ApproveAliasResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetCanonicalItemId() == "" || req.Msg.GetAliasItemId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id, canonical_item_id, and alias_item_id are required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.ApproveAlias(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetCanonicalItemId(), req.Msg.GetAliasItemId(), userID); err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.ApproveAliasResponse{
		CanonicalItemId: req.Msg.GetCanonicalItemId(),
		AliasItemId:     req.Msg.GetAliasItemId(),
		MergeStatus:     "approved",
	}), nil
}

func (h *ItemHandler) RejectAlias(ctx context.Context, req *connect.Request[appv1.RejectAliasRequest]) (*connect.Response[appv1.RejectAliasResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetCanonicalItemId() == "" || req.Msg.GetAliasItemId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id, canonical_item_id, and alias_item_id are required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.RejectAlias(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetCanonicalItemId(), req.Msg.GetAliasItemId(), userID); err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.RejectAliasResponse{
		CanonicalItemId: req.Msg.GetCanonicalItemId(),
		AliasItemId:     req.Msg.GetAliasItemId(),
		MergeStatus:     "rejected",
	}), nil
}
