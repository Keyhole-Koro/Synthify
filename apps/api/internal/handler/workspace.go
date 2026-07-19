package handler

import (
	"context"
	"errors"
	"strings"

	connect "connectrpc.com/connect"
	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type WorkspaceHandler struct {
	service WorkspaceUsecase
}

func NewWorkspaceHandler(svc WorkspaceUsecase) *WorkspaceHandler {
	return &WorkspaceHandler{service: svc}
}

func (h *WorkspaceHandler) ListWorkspaces(ctx context.Context, _ *connect.Request[appv1.ListWorkspacesRequest]) (*connect.Response[appv1.ListWorkspacesResponse], error) {
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	workspaces, err := h.service.ListWorkspaces(ctx, userID)
	if err != nil {
		return nil, toError(err)
	}
	res := connect.NewResponse(&appv1.ListWorkspacesResponse{})
	for _, workspace := range workspaces {
		res.Msg.Workspaces = append(res.Msg.Workspaces, toProtoWorkspace(workspace))
	}
	return res, nil
}

func (h *WorkspaceHandler) GetWorkspace(ctx context.Context, req *connect.Request[appv1.GetWorkspaceRequest]) (*connect.Response[appv1.GetWorkspaceResponse], error) {
	if req.Msg.GetWorkspaceId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	workspace, err := h.service.GetWorkspace(ctx, req.Msg.GetWorkspaceId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	members, err := h.service.ListMembers(ctx, req.Msg.GetWorkspaceId(), userID)
	if err != nil {
		return nil, toError(err)
	}
	res := &appv1.GetWorkspaceResponse{
		Workspace: toProtoWorkspace(workspace),
	}
	for _, m := range members {
		res.Members = append(res.Members, toProtoWorkspaceMember(m))
	}
	return connect.NewResponse(res), nil
}

func (h *WorkspaceHandler) CreateWorkspace(ctx context.Context, req *connect.Request[appv1.CreateWorkspaceRequest]) (*connect.Response[appv1.CreateWorkspaceResponse], error) {
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := h.service.CreateWorkspace(ctx, name, userID)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.CreateWorkspaceResponse{
		Workspace: toProtoWorkspace(ws),
	}), nil
}

func (h *WorkspaceHandler) UpdateWorkspace(ctx context.Context, req *connect.Request[appv1.UpdateWorkspaceRequest]) (*connect.Response[appv1.UpdateWorkspaceResponse], error) {
	workspaceID := strings.TrimSpace(req.Msg.GetWorkspaceId())
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := h.service.UpdateWorkspace(ctx, workspaceID, name, userID)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.UpdateWorkspaceResponse{
		Workspace: toProtoWorkspace(ws),
	}), nil
}

func (h *WorkspaceHandler) DeleteWorkspace(ctx context.Context, req *connect.Request[appv1.DeleteWorkspaceRequest]) (*connect.Response[appv1.DeleteWorkspaceResponse], error) {
	workspaceID := strings.TrimSpace(req.Msg.GetWorkspaceId())
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.DeleteWorkspace(ctx, workspaceID, userID); err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.DeleteWorkspaceResponse{}), nil
}

func (h *WorkspaceHandler) InviteMember(ctx context.Context, req *connect.Request[appv1.InviteMemberRequest]) (*connect.Response[appv1.InviteMemberResponse], error) {
	workspaceID := strings.TrimSpace(req.Msg.GetWorkspaceId())
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	email := strings.TrimSpace(req.Msg.GetEmail())
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	member, err := h.service.InviteMember(ctx, workspaceID, userID, email, fromProtoWorkspaceRole(req.Msg.GetRole()))
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.InviteMemberResponse{
		Member: toProtoWorkspaceMember(member),
	}), nil
}

func (h *WorkspaceHandler) UpdateMemberRole(ctx context.Context, req *connect.Request[appv1.UpdateMemberRoleRequest]) (*connect.Response[appv1.UpdateMemberRoleResponse], error) {
	workspaceID := strings.TrimSpace(req.Msg.GetWorkspaceId())
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	targetUserID := strings.TrimSpace(req.Msg.GetUserId())
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	member, err := h.service.UpdateMemberRole(ctx, workspaceID, userID, targetUserID, fromProtoWorkspaceRole(req.Msg.GetRole()))
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.UpdateMemberRoleResponse{
		Member: toProtoWorkspaceMember(member),
	}), nil
}

func (h *WorkspaceHandler) RemoveMember(ctx context.Context, req *connect.Request[appv1.RemoveMemberRequest]) (*connect.Response[appv1.RemoveMemberResponse], error) {
	workspaceID := strings.TrimSpace(req.Msg.GetWorkspaceId())
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	targetUserID := strings.TrimSpace(req.Msg.GetUserId())
	if targetUserID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.RemoveMember(ctx, workspaceID, userID, targetUserID); err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.RemoveMemberResponse{}), nil
}

func (h *WorkspaceHandler) TransferOwnership(_ context.Context, _ *connect.Request[appv1.TransferOwnershipRequest]) (*connect.Response[appv1.TransferOwnershipResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("workspace ownership is managed at account level"))
}

func (h *WorkspaceHandler) CreateShareLink(ctx context.Context, req *connect.Request[appv1.CreateShareLinkRequest]) (*connect.Response[appv1.CreateShareLinkResponse], error) {
	workspaceID := strings.TrimSpace(req.Msg.GetWorkspaceId())
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	link, err := h.service.CreateShareLink(ctx, workspaceID, userID, fromProtoWorkspaceRole(req.Msg.GetRole()), strings.TrimSpace(req.Msg.GetExpiresAt()))
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.CreateShareLinkResponse{
		Link: toProtoShareLink(link),
	}), nil
}

func (h *WorkspaceHandler) ListShareLinks(ctx context.Context, req *connect.Request[appv1.ListShareLinksRequest]) (*connect.Response[appv1.ListShareLinksResponse], error) {
	workspaceID := strings.TrimSpace(req.Msg.GetWorkspaceId())
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	links, err := h.service.ListShareLinks(ctx, workspaceID, userID)
	if err != nil {
		return nil, toError(err)
	}
	res := &appv1.ListShareLinksResponse{}
	for _, l := range links {
		res.Links = append(res.Links, toProtoShareLink(l))
	}
	return connect.NewResponse(res), nil
}

func (h *WorkspaceHandler) RevokeShareLink(ctx context.Context, req *connect.Request[appv1.RevokeShareLinkRequest]) (*connect.Response[appv1.RevokeShareLinkResponse], error) {
	workspaceID := strings.TrimSpace(req.Msg.GetWorkspaceId())
	if workspaceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace_id is required"))
	}
	token := strings.TrimSpace(req.Msg.GetToken())
	if token == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.service.RevokeShareLink(ctx, workspaceID, userID, token); err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.RevokeShareLinkResponse{}), nil
}

// ResolveShareLink は無認証経路 (auth middleware で exempt)。token のみで解決する。
func (h *WorkspaceHandler) ResolveShareLink(ctx context.Context, req *connect.Request[appv1.ResolveShareLinkRequest]) (*connect.Response[appv1.ResolveShareLinkResponse], error) {
	token := strings.TrimSpace(req.Msg.GetToken())
	if token == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is required"))
	}
	workspace, role, err := h.service.ResolveShareLink(ctx, token)
	if err != nil {
		return nil, toError(err)
	}
	return connect.NewResponse(&appv1.ResolveShareLinkResponse{
		Workspace: toProtoWorkspace(workspace),
		Role:      toProtoWorkspaceRole(role),
	}), nil
}

func toProtoWorkspace(ws *domain.Workspace) *appv1.Workspace {
	if ws == nil {
		return nil
	}
	return &appv1.Workspace{
		WorkspaceId:       ws.WorkspaceID,
		Name:              ws.Name,
		OwnerId:           ws.AccountID,
		Plan:              toProtoWorkspacePlan(ws.Plan),
		StorageUsedBytes:  ws.StorageUsedBytes,
		StorageQuotaBytes: ws.StorageQuotaBytes,
		MaxFileSizeBytes:  ws.MaxFileSizeBytes,
		MaxUploadsPerDay:  ws.MaxUploadsPerWeek,
		CreatedAt:         ws.CreatedAt,
	}
}

func toProtoWorkspaceMember(m *domain.WorkspaceMember) *appv1.WorkspaceMember {
	if m == nil {
		return nil
	}
	return &appv1.WorkspaceMember{
		UserId:    m.UserID,
		Email:     m.Email,
		Role:      toProtoWorkspaceRole(m.Role),
		InvitedAt: m.InvitedAt,
		InvitedBy: m.InvitedBy,
	}
}

func toProtoShareLink(l *domain.ShareLink) *appv1.ShareLink {
	if l == nil {
		return nil
	}
	return &appv1.ShareLink{
		Token:       l.Token,
		WorkspaceId: l.WorkspaceID,
		Role:        toProtoWorkspaceRole(l.Role),
		CreatedBy:   l.CreatedBy,
		CreatedAt:   l.CreatedAt,
		ExpiresAt:   l.ExpiresAt,
		Revoked:     l.Revoked,
	}
}

func toProtoWorkspaceRole(role domain.WorkspaceRole) appv1.WorkspaceRole {
	switch role {
	case domain.WorkspaceRoleOwner:
		return appv1.WorkspaceRole_WORKSPACE_ROLE_OWNER
	case domain.WorkspaceRoleEditor:
		return appv1.WorkspaceRole_WORKSPACE_ROLE_EDITOR
	case domain.WorkspaceRoleViewer:
		return appv1.WorkspaceRole_WORKSPACE_ROLE_VIEWER
	default:
		return appv1.WorkspaceRole_WORKSPACE_ROLE_UNSPECIFIED
	}
}

func fromProtoWorkspaceRole(role appv1.WorkspaceRole) domain.WorkspaceRole {
	switch role {
	case appv1.WorkspaceRole_WORKSPACE_ROLE_OWNER:
		return domain.WorkspaceRoleOwner
	case appv1.WorkspaceRole_WORKSPACE_ROLE_EDITOR:
		return domain.WorkspaceRoleEditor
	case appv1.WorkspaceRole_WORKSPACE_ROLE_VIEWER:
		return domain.WorkspaceRoleViewer
	default:
		return ""
	}
}

func toProtoWorkspacePlan(plan string) appv1.WorkspacePlan {
	switch plan {
	case "pro":
		return appv1.WorkspacePlan_WORKSPACE_PLAN_PRO
	case "free", "":
		return appv1.WorkspacePlan_WORKSPACE_PLAN_FREE
	default:
		return appv1.WorkspacePlan_WORKSPACE_PLAN_UNSPECIFIED
	}
}
