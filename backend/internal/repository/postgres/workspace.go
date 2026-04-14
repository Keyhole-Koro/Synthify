package postgres

import (
	"context"
	"time"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

func (s *Store) ListWorkspaces() []*domain.Workspace {
	rows, err := s.q().ListWorkspaces(context.Background())
	if err != nil {
		return nil
	}

	var workspaces []*domain.Workspace
	for _, row := range rows {
		workspaces = append(workspaces, toWorkspace(row))
	}
	return workspaces
}

func (s *Store) ListWorkspacesByUser(userID string) []*domain.Workspace {
	rows, err := s.q().ListWorkspacesByUser(context.Background(), userID)
	if err != nil {
		return nil
	}

	var workspaces []*domain.Workspace
	for _, row := range rows {
		workspaces = append(workspaces, toWorkspace(row))
	}
	return workspaces
}

func (s *Store) GetWorkspace(id string) (*domain.Workspace, []*domain.WorkspaceMember, bool) {
	ctx := context.Background()
	row, err := s.q().GetWorkspace(ctx, id)
	if err != nil {
		return nil, nil, false
	}
	membersRows, err := s.q().ListWorkspaceMembers(ctx, id)
	if err != nil {
		return nil, nil, false
	}

	var members []*domain.WorkspaceMember
	for _, memberRow := range membersRows {
		members = append(members, toWorkspaceMember(memberRow))
	}
	return toWorkspace(row), members, true
}

func (s *Store) IsWorkspaceMember(wsID, userID string) bool {
	_, err := s.q().GetWorkspaceMember(context.Background(), sqlcgen.GetWorkspaceMemberParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	return err == nil
}

func (s *Store) CreateWorkspace(name, ownerUserID, ownerEmail string) *domain.Workspace {
	createdAt := nowTime()
	workspace := &domain.Workspace{
		WorkspaceID:       newID("ws"),
		Name:              name,
		OwnerID:           ownerUserID,
		Plan:              "free",
		StorageUsedBytes:  0,
		StorageQuotaBytes: 1 << 30,
		MaxFileSizeBytes:  50 << 20,
		MaxUploadsPerDay:  10,
		CreatedAt:         createdAt.Format(time.RFC3339),
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil
	}
	defer tx.Rollback()
	qtx := s.q().WithTx(tx)
	ctx := context.Background()

	if err := qtx.CreateWorkspace(ctx, sqlcgen.CreateWorkspaceParams{
		WorkspaceID:       workspace.WorkspaceID,
		Name:              workspace.Name,
		OwnerID:           workspace.OwnerID,
		Plan:              workspace.Plan,
		StorageUsedBytes:  workspace.StorageUsedBytes,
		StorageQuotaBytes: workspace.StorageQuotaBytes,
		MaxFileSizeBytes:  workspace.MaxFileSizeBytes,
		MaxUploadsPerDay:  workspace.MaxUploadsPerDay,
		CreatedAt:         createdAt,
	}); err != nil {
		return nil
	}
	if err := qtx.CreateWorkspaceMember(ctx, sqlcgen.CreateWorkspaceMemberParams{
		WorkspaceID: workspace.WorkspaceID,
		UserID:      ownerUserID,
		Email:       ownerEmail,
		Role:        "owner",
		IsDev:       true,
		InvitedAt:   createdAt,
		InvitedBy:   ownerUserID,
	}); err != nil {
		return nil
	}
	if err := tx.Commit(); err != nil {
		return nil
	}
	return workspace
}

func (s *Store) InviteMember(wsID, email, role string, isDev bool) (*domain.WorkspaceMember, bool) {
	invitedAt := nowTime()
	member := &domain.WorkspaceMember{
		UserID:    newID("user"),
		Email:     email,
		Role:      domain.WorkspaceRole(role),
		IsDev:     isDev,
		InvitedAt: invitedAt.Format(time.RFC3339),
		InvitedBy: "user_demo",
	}
	err := s.q().CreateWorkspaceMember(context.Background(), sqlcgen.CreateWorkspaceMemberParams{
		WorkspaceID: wsID,
		UserID:      member.UserID,
		Email:       member.Email,
		Role:        string(member.Role),
		IsDev:       member.IsDev,
		InvitedAt:   invitedAt,
		InvitedBy:   member.InvitedBy,
	})
	return member, err == nil
}

func (s *Store) UpdateMemberRole(wsID, userID, role string, isDev bool) (*domain.WorkspaceMember, bool) {
	ctx := context.Background()
	affected, err := s.q().UpdateWorkspaceMemberRole(ctx, sqlcgen.UpdateWorkspaceMemberRoleParams{
		WorkspaceID: wsID,
		UserID:      userID,
		Role:        role,
		IsDev:       isDev,
	})
	if err != nil {
		return nil, false
	}
	if affected == 0 {
		return nil, false
	}
	memberRow, err := s.q().GetWorkspaceMember(ctx, sqlcgen.GetWorkspaceMemberParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	if err != nil {
		return nil, false
	}
	return toWorkspaceMemberFromGet(memberRow), true
}

func (s *Store) RemoveMember(wsID, userID string) bool {
	affected, err := s.q().DeleteWorkspaceMember(context.Background(), sqlcgen.DeleteWorkspaceMemberParams{
		WorkspaceID: wsID,
		UserID:      userID,
	})
	if err != nil {
		return false
	}
	return affected > 0
}

func (s *Store) TransferOwnership(wsID, newOwnerUserID string) (*domain.Workspace, []*domain.WorkspaceMember, bool) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, false
	}
	defer tx.Rollback()
	qtx := s.q().WithTx(tx)
	ctx := context.Background()

	oldOwner, err := qtx.GetWorkspaceOwnerID(ctx, wsID)
	if err != nil {
		return nil, nil, false
	}
	affected, err := qtx.PromoteWorkspaceOwner(ctx, sqlcgen.PromoteWorkspaceOwnerParams{
		WorkspaceID: wsID,
		UserID:      newOwnerUserID,
	})
	if err != nil {
		return nil, nil, false
	}
	if affected == 0 {
		return nil, nil, false
	}
	if err := qtx.DemoteWorkspaceOwner(ctx, sqlcgen.DemoteWorkspaceOwnerParams{
		WorkspaceID: wsID,
		UserID:      oldOwner,
	}); err != nil {
		return nil, nil, false
	}
	if err := qtx.UpdateWorkspaceOwner(ctx, sqlcgen.UpdateWorkspaceOwnerParams{
		WorkspaceID: wsID,
		OwnerID:     newOwnerUserID,
	}); err != nil {
		return nil, nil, false
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, false
	}
	return s.GetWorkspace(wsID)
}
