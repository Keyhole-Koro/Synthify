package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/apps/api/internal/service"
	"github.com/synthify/backend/apps/api/internal/transport/connect"
	"github.com/synthify/backend/apps/api/internal/transport/connect/mappers"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type WorkspaceHandler struct {
	service    *service.WorkspaceService
	billing    service.BillingUsecase
	workspaces repository.WorkspaceRepository
}

func NewWorkspaceHandler(svc *service.WorkspaceService, billing service.BillingUsecase, workspaceRepo repository.WorkspaceRepository) *WorkspaceHandler {
	return &WorkspaceHandler{service: svc, billing: billing, workspaces: workspaceRepo}
}

func (h *WorkspaceHandler) ListWorkspaces(ctx context.Context, _ *connect.Request[appv1.ListWorkspacesRequest]) (*connect.Response[appv1.ListWorkspacesResponse], error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	workspaces := h.workspaces.ListWorkspacesByUser(ctx, user.ID)
	res := connect.NewResponse(&appv1.ListWorkspacesResponse{})
	for _, workspace := range workspaces {
		res.Msg.Workspaces = append(res.Msg.Workspaces, mappers.ToProtoWorkspace(workspace))
	}
	return res, nil
}

func (h *WorkspaceHandler) GetWorkspace(ctx context.Context, req *connect.Request[appv1.GetWorkspaceRequest]) (*connect.Response[appv1.GetWorkspaceResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	workspace, err := h.service.GetWorkspace(ctx, req.Msg.GetWorkspaceId(), user.ID)
	if err != nil {
		return nil, connectutil.ToError(err)
	}
	return connect.NewResponse(&appv1.GetWorkspaceResponse{
		Workspace: mappers.ToProtoWorkspace(workspace),
	}), nil
}

func (h *WorkspaceHandler) CreateWorkspace(ctx context.Context, req *connect.Request[appv1.CreateWorkspaceRequest]) (*connect.Response[appv1.CreateWorkspaceResponse], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := h.service.CreateWorkspace(ctx, req.Msg.GetName(), user.ID)
	if err != nil {
		return nil, connectutil.ToError(err)
	}
	// サインアップ時の無料クレジット付与（冪等なので毎回呼んで問題なし）
	if h.billing != nil {
		_ = h.billing.GrantFreeSignupCredit(ctx, ws.AccountID)
	}
	return connect.NewResponse(&appv1.CreateWorkspaceResponse{
		Workspace: mappers.ToProtoWorkspace(ws),
	}), nil
}

func (h *WorkspaceHandler) InviteMember(_ context.Context, _ *connect.Request[appv1.InviteMemberRequest]) (*connect.Response[appv1.InviteMemberResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("workspace membership is managed at account level"))
}

func (h *WorkspaceHandler) UpdateMemberRole(_ context.Context, _ *connect.Request[appv1.UpdateMemberRoleRequest]) (*connect.Response[appv1.UpdateMemberRoleResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("workspace membership is managed at account level"))
}

func (h *WorkspaceHandler) RemoveMember(_ context.Context, _ *connect.Request[appv1.RemoveMemberRequest]) (*connect.Response[appv1.RemoveMemberResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("workspace membership is managed at account level"))
}

func (h *WorkspaceHandler) TransferOwnership(_ context.Context, _ *connect.Request[appv1.TransferOwnershipRequest]) (*connect.Response[appv1.TransferOwnershipResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("workspace ownership is managed at account level"))
}
