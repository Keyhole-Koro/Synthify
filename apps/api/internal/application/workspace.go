package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// WorkspaceUsecase defines the use-case interface for workspace management, memberships, and share links.
type WorkspaceUsecase interface {
	ListWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error)
	GetWorkspace(ctx context.Context, id, userID string) (*domain.Workspace, error)
	CreateWorkspace(ctx context.Context, name, userID string) (*domain.Workspace, error)
	UpdateWorkspace(ctx context.Context, id, name, userID string) (*domain.Workspace, error)
	DeleteWorkspace(ctx context.Context, id, userID string) error
	ListMembers(ctx context.Context, wsID, userID string) ([]*domain.WorkspaceMember, error)
	InviteMember(ctx context.Context, wsID, userID, email string, role domain.WorkspaceRole) (*domain.WorkspaceMember, error)
	UpdateMemberRole(ctx context.Context, wsID, userID, targetUserID string, role domain.WorkspaceRole) (*domain.WorkspaceMember, error)
	RemoveMember(ctx context.Context, wsID, userID, targetUserID string) error
	CreateShareLink(ctx context.Context, wsID, userID string, role domain.WorkspaceRole, expiresAt string) (*domain.ShareLink, error)
	ListShareLinks(ctx context.Context, wsID, userID string) ([]*domain.ShareLink, error)
	RevokeShareLink(ctx context.Context, wsID, userID, token string) error
	// ResolveShareLink resolves an unauthenticated public share token to a workspace and role.
	ResolveShareLink(ctx context.Context, token string) (*domain.Workspace, domain.WorkspaceRole, error)
}

// WorkspaceService manages workspace lifecycle, collaborative memberships, and invitation links.
type WorkspaceService struct {
	accounts   repository.AccountRepository
	workspaces repository.WorkspaceRepository
	users      repository.UserRepository
	logger     *slog.Logger
}

// WorkspaceServiceDeps defines dependencies required to instantiate WorkspaceService.
type WorkspaceServiceDeps struct {
	Accounts   repository.AccountRepository
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Logger     *slog.Logger
}

// NewWorkspaceService creates a new WorkspaceService instance.
func NewWorkspaceService(deps WorkspaceServiceDeps) *WorkspaceService {
	return &WorkspaceService{
		accounts:   deps.Accounts,
		workspaces: deps.Workspaces,
		users:      deps.Users,
		logger:     deps.Logger,
	}
}

// ListWorkspaces lists all workspaces accessible by the specified user.
func (s *WorkspaceService) ListWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	return s.workspaces.ListWorkspacesByUser(ctx, userID)
}

// GetWorkspace retrieves a workspace by ID after verifying user access.
func (s *WorkspaceService) GetWorkspace(ctx context.Context, id, userID string) (*domain.Workspace, error) {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	ws, err := s.workspaces.GetWorkspace(ctx, id)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

// CreateWorkspace provisions a new workspace under the user's primary account.
func (s *WorkspaceService) CreateWorkspace(ctx context.Context, name, userID string) (*domain.Workspace, error) {
	account, err := s.accounts.GetAccountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.workspaces.CreateWorkspace(ctx, account.AccountID, name)
}

// UpdateWorkspace modifies the name of an existing workspace if the user has editor or owner permissions.
func (s *WorkspaceService) UpdateWorkspace(ctx context.Context, id, name, userID string) (*domain.Workspace, error) {
	role, err := s.workspaces.GetWorkspaceRole(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !role.CanWrite() {
		return nil, domain.ErrForbidden
	}
	ws, err := s.workspaces.UpdateWorkspaceName(ctx, id, name)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

// DeleteWorkspace soft-deletes a workspace if the requesting user has owner permissions.
func (s *WorkspaceService) DeleteWorkspace(ctx context.Context, id, userID string) error {
	role, err := s.workspaces.GetWorkspaceRole(ctx, id, userID)
	if err != nil {
		return err
	}
	if !role.CanManageMembers() {
		return domain.ErrForbidden
	}
	return s.workspaces.DeleteWorkspace(ctx, id)
}

// requireManageMembers verifies that the user holds owner permissions to manage members.
func (s *WorkspaceService) requireManageMembers(ctx context.Context, wsID, userID string) error {
	role, err := s.workspaces.GetWorkspaceRole(ctx, wsID, userID)
	if err != nil {
		return err
	}
	if !role.CanManageMembers() {
		return domain.ErrForbidden
	}
	return nil
}

// ListMembers returns all users and their roles belonging to the workspace.
func (s *WorkspaceService) ListMembers(ctx context.Context, wsID, userID string) ([]*domain.WorkspaceMember, error) {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, wsID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	return s.workspaces.ListWorkspaceMembers(ctx, wsID)
}

// InviteMember adds an existing registered user to the workspace with the specified role.
func (s *WorkspaceService) InviteMember(ctx context.Context, wsID, userID, email string, role domain.WorkspaceRole) (*domain.WorkspaceMember, error) {
	if !isAssignableRole(role) {
		return nil, domain.ErrInvalidArgument
	}
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return nil, err
	}
	// Invitations require an existing user profile; returns ErrNotFound if unregistered.
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if err := s.workspaces.UpsertWorkspaceMember(ctx, wsID, user.UserID, role, userID); err != nil {
		return nil, err
	}
	return &domain.WorkspaceMember{
		WorkspaceID: wsID,
		UserID:      user.UserID,
		Email:       user.Email,
		Role:        role,
		InvitedBy:   userID,
	}, nil
}

// UpdateMemberRole modifies the assigned role of an existing workspace member.
func (s *WorkspaceService) UpdateMemberRole(ctx context.Context, wsID, userID, targetUserID string, role domain.WorkspaceRole) (*domain.WorkspaceMember, error) {
	if !isAssignableRole(role) {
		return nil, domain.ErrInvalidArgument
	}
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return nil, err
	}
	if err := s.workspaces.UpsertWorkspaceMember(ctx, wsID, targetUserID, role, userID); err != nil {
		return nil, err
	}
	user, err := s.users.GetUser(ctx, targetUserID)
	if err != nil {
		return nil, err
	}
	return &domain.WorkspaceMember{
		WorkspaceID: wsID,
		UserID:      targetUserID,
		Email:       user.Email,
		Role:        role,
		InvitedBy:   userID,
	}, nil
}

// RemoveMember removes a member from the workspace.
func (s *WorkspaceService) RemoveMember(ctx context.Context, wsID, userID, targetUserID string) error {
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return err
	}
	return s.workspaces.RemoveWorkspaceMember(ctx, wsID, targetUserID)
}

// isAssignableRole validates whether a role can be assigned via invitations or role updates.
// The owner role is reserved for the primary account holder and cannot be assigned explicitly.
func isAssignableRole(role domain.WorkspaceRole) bool {
	return role == domain.WorkspaceRoleEditor || role == domain.WorkspaceRoleViewer
}

// CreateShareLink generates a new invite share link for the workspace with a cryptographically secure token.
func (s *WorkspaceService) CreateShareLink(ctx context.Context, wsID, userID string, role domain.WorkspaceRole, expiresAt string) (*domain.ShareLink, error) {
	if role == "" {
		role = domain.WorkspaceRoleViewer
	}
	if !isAssignableRole(role) {
		return nil, domain.ErrInvalidArgument
	}
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return nil, err
	}
	token, err := generateShareToken()
	if err != nil {
		return nil, err
	}
	link := &domain.ShareLink{
		Token:       token,
		WorkspaceID: wsID,
		Role:        role,
		CreatedBy:   userID,
		ExpiresAt:   expiresAt,
	}
	if err := s.workspaces.CreateShareLink(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// ListShareLinks returns all share links issued for a workspace.
func (s *WorkspaceService) ListShareLinks(ctx context.Context, wsID, userID string) ([]*domain.ShareLink, error) {
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return nil, err
	}
	return s.workspaces.ListShareLinks(ctx, wsID)
}

// RevokeShareLink marks a workspace share link token as revoked.
func (s *WorkspaceService) RevokeShareLink(ctx context.Context, wsID, userID, token string) error {
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return err
	}
	return s.workspaces.RevokeShareLink(ctx, wsID, token, time.Now().UTC())
}

// ResolveShareLink resolves a share token to its target workspace and permitted role without user authentication.
func (s *WorkspaceService) ResolveShareLink(ctx context.Context, token string) (*domain.Workspace, domain.WorkspaceRole, error) {
	link, err := s.workspaces.ResolveShareLink(ctx, token, time.Now().UTC())
	if err != nil {
		return nil, "", err
	}
	ws, err := s.workspaces.GetWorkspace(ctx, link.WorkspaceID)
	if err != nil {
		return nil, "", err
	}
	return ws, link.Role, nil
}

// generateShareToken generates a cryptographically random, URL-safe 256-bit token.
func generateShareToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
