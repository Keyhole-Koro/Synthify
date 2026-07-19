package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// ItemUsecase is the application API required by ItemHandler.
// The consumer owns this interface so the application layer does not need to
// define an interface for its own concrete implementation.
type ItemUsecase interface {
	GetItem(ctx context.Context, itemID, workspaceID, userID string) (*domain.Item, error)
	GetItemEvidence(ctx context.Context, itemID, workspaceID, userID string) (*domain.ItemEvidence, error)
	CreateItem(ctx context.Context, workspaceID, label, description, parentID, userID string) (*domain.Item, error)
	ApproveAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error
	RejectAlias(ctx context.Context, workspaceID, canonicalItemID, aliasItemID, userID string) error
}

type ItemHandler struct {
	service ItemUsecase
}

func NewItemHandler(service ItemUsecase) *ItemHandler {
	return &ItemHandler{service: service}
}

func (h *ItemHandler) GetTreeEntityDetail(ctx context.Context, req *connect.Request[appv1.GetTreeEntityDetailRequest]) (*connect.Response[appv1.GetTreeEntityDetailResponse], error) {
	if req.Msg.GetTargetRef() == nil || req.Msg.GetTargetRef().GetId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_ref.id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	item, err := h.application.GetItem(ctx, req.Msg.GetTargetRef().GetId(), req.Msg.GetTargetRef().GetWorkspaceId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	evidence, err := h.application.GetItemEvidence(ctx, item.ItemID, req.Msg.GetTargetRef().GetWorkspaceId(), userID)
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
	item, err := h.application.CreateItem(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetLabel(), req.Msg.GetDescription(), req.Msg.GetParentId(), userID)
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
	if err := h.application.ApproveAlias(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetCanonicalItemId(), req.Msg.GetAliasItemId(), userID); err != nil {
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
	if err := h.application.RejectAlias(ctx, req.Msg.GetWorkspaceId(), req.Msg.GetCanonicalItemId(), req.Msg.GetAliasItemId(), userID); err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.RejectAliasResponse{
		CanonicalItemId: req.Msg.GetCanonicalItemId(),
		AliasItemId:     req.Msg.GetAliasItemId(),
		MergeStatus:     "rejected",
	}), nil
}
