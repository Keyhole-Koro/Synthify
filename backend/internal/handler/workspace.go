package handler

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	graphv1 "github.com/synthify/backend/gen/synthify/graph/v1"
	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/service"
)

type WorkspaceHandler struct {
	service *service.WorkspaceService
}

func NewWorkspaceHandler(svc *service.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{service: svc}
}

func (h *WorkspaceHandler) ListWorkspaces(ctx context.Context, _ *connect.Request[graphv1.ListWorkspacesRequest]) (*connect.Response[graphv1.ListWorkspacesResponse], error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	workspaces := h.service.ListWorkspaces(user.ID)
	res := connect.NewResponse(&graphv1.ListWorkspacesResponse{})
	for _, workspace := range workspaces {
		res.Msg.Workspaces = append(res.Msg.Workspaces, toProtoWorkspace(workspace))
	}
	return res, nil
}

func (h *WorkspaceHandler) GetWorkspace(ctx context.Context, req *connect.Request[graphv1.GetWorkspaceRequest]) (*connect.Response[graphv1.GetWorkspaceResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	workspace, members, err := h.service.GetWorkspace(req.Msg.GetWorkspaceId(), user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	res := connect.NewResponse(&graphv1.GetWorkspaceResponse{
		Workspace: toProtoWorkspace(workspace),
	})
	for _, member := range members {
		res.Msg.Members = append(res.Msg.Members, toProtoWorkspaceMember(member))
	}
	return res, nil
}

func (h *WorkspaceHandler) CreateWorkspace(ctx context.Context, req *connect.Request[graphv1.CreateWorkspaceRequest]) (*connect.Response[graphv1.CreateWorkspaceResponse], error) {
	if req.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	email := user.Email
	if email == "" {
		email = user.ID + "@local.invalid"
	}
	return connect.NewResponse(&graphv1.CreateWorkspaceResponse{
		Workspace: toProtoWorkspace(h.service.CreateWorkspace(req.Msg.GetName(), user.ID, email)),
	}), nil
}

func (h *WorkspaceHandler) InviteMember(_ context.Context, req *connect.Request[graphv1.InviteMemberRequest]) (*connect.Response[graphv1.InviteMemberResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetEmail() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and email are required"))
	}
	member, err := h.service.InviteMember(req.Msg.GetWorkspaceId(), req.Msg.GetEmail(), workspaceRoleToDomain(req.Msg.GetRole()), req.Msg.GetIsDev())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&graphv1.InviteMemberResponse{Member: toProtoWorkspaceMember(member)}), nil
}

func (h *WorkspaceHandler) UpdateMemberRole(_ context.Context, req *connect.Request[graphv1.UpdateMemberRoleRequest]) (*connect.Response[graphv1.UpdateMemberRoleResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and user_id are required"))
	}
	member, err := h.service.UpdateMemberRole(req.Msg.GetWorkspaceId(), req.Msg.GetUserId(), workspaceRoleToDomain(req.Msg.GetRole()), req.Msg.GetIsDev())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&graphv1.UpdateMemberRoleResponse{Member: toProtoWorkspaceMember(member)}), nil
}

func (h *WorkspaceHandler) RemoveMember(_ context.Context, req *connect.Request[graphv1.RemoveMemberRequest]) (*connect.Response[graphv1.RemoveMemberResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and user_id are required"))
	}
	if err := h.service.RemoveMember(req.Msg.GetWorkspaceId(), req.Msg.GetUserId()); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(&graphv1.RemoveMemberResponse{}), nil
}

func (h *WorkspaceHandler) TransferOwnership(_ context.Context, req *connect.Request[graphv1.TransferOwnershipRequest]) (*connect.Response[graphv1.TransferOwnershipResponse], error) {
	if req.Msg.GetWorkspaceId() == "" || req.Msg.GetNewOwnerUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id and new_owner_user_id are required"))
	}
	workspace, members, err := h.service.TransferOwnership(req.Msg.GetWorkspaceId(), req.Msg.GetNewOwnerUserId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	res := connect.NewResponse(&graphv1.TransferOwnershipResponse{Workspace: toProtoWorkspace(workspace)})
	for _, member := range members {
		res.Msg.Members = append(res.Msg.Members, toProtoWorkspaceMember(member))
	}
	return res, nil
}

func toProtoWorkspace(workspace *domain.Workspace) *graphv1.Workspace {
	return &graphv1.Workspace{
		WorkspaceId:       workspace.WorkspaceID,
		Name:              workspace.Name,
		OwnerId:           workspace.OwnerID,
		Plan:              workspacePlanToProto(workspace.Plan),
		StorageUsedBytes:  workspace.StorageUsedBytes,
		StorageQuotaBytes: workspace.StorageQuotaBytes,
		MaxFileSizeBytes:  workspace.MaxFileSizeBytes,
		MaxUploadsPerDay:  workspace.MaxUploadsPerDay,
		CreatedAt:         workspace.CreatedAt,
	}
}

func toProtoWorkspaceMember(member *domain.WorkspaceMember) *graphv1.WorkspaceMember {
	return &graphv1.WorkspaceMember{
		UserId:    member.UserID,
		Email:     member.Email,
		Role:      workspaceRoleToProto(member.Role),
		IsDev:     member.IsDev,
		InvitedAt: member.InvitedAt,
		InvitedBy: member.InvitedBy,
	}
}
