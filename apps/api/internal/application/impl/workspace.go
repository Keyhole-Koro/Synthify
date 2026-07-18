package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository"
)

// WorkspaceUsecase は handler が依存する WorkspaceService の API 表面。
// ListWorkspaces は PR 4 (認可 Service 移動) で handler の直接 repository call を
// この interface 経由に切り替える前提でここに含める。
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
	// ResolveShareLink は無認証で呼ばれる。有効な token なら workspace と role を返す。
	ResolveShareLink(ctx context.Context, token string) (*domain.Workspace, domain.WorkspaceRole, error)
}

type WorkspaceService struct {
	accounts   repository.AccountRepository
	workspaces repository.WorkspaceRepository
	users      repository.UserRepository
	logger     *slog.Logger
}

type WorkspaceServiceDeps struct {
	Accounts   repository.AccountRepository
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Logger     *slog.Logger
}

func NewWorkspaceService(deps WorkspaceServiceDeps) *WorkspaceService {
	return &WorkspaceService{
		accounts:   deps.Accounts,
		workspaces: deps.Workspaces,
		users:      deps.Users,
		logger:     deps.Logger,
	}
}

func (s *WorkspaceService) ListWorkspaces(ctx context.Context, userID string) ([]*domain.Workspace, error) {
	return s.workspaces.ListWorkspacesByUser(ctx, userID)
}

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

func (s *WorkspaceService) CreateWorkspace(ctx context.Context, name, userID string) (*domain.Workspace, error) {
	account, err := s.accounts.GetAccountByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.workspaces.CreateWorkspace(ctx, account.AccountID, name)
}

func (s *WorkspaceService) UpdateWorkspace(ctx context.Context, id, name, userID string) (*domain.Workspace, error) {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrForbidden
	}
	ws, err := s.workspaces.UpdateWorkspaceName(ctx, id, name)
	if err != nil {
		return nil, err
	}
	return ws, nil
}

func (s *WorkspaceService) DeleteWorkspace(ctx context.Context, id, userID string) error {
	ok, err := s.workspaces.IsWorkspaceAccessible(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return s.workspaces.DeleteWorkspace(ctx, id)
}

// requireManageMembers はメンバー管理操作 (招待 / role 変更 / 削除) の権限を確認する。
// owner だけが管理でき、それ以外 (editor / viewer / 非メンバー) は ErrForbidden。
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

func (s *WorkspaceService) InviteMember(ctx context.Context, wsID, userID, email string, role domain.WorkspaceRole) (*domain.WorkspaceMember, error) {
	if !isAssignableRole(role) {
		return nil, domain.ErrInvalidArgument
	}
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return nil, err
	}
	// 課金が生じる操作は登録済みユーザーのみに許可する方針のため、
	// 招待は users に存在する (= 登録済み) ユーザーに限る。未登録は ErrNotFound。
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

func (s *WorkspaceService) RemoveMember(ctx context.Context, wsID, userID, targetUserID string) error {
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return err
	}
	return s.workspaces.RemoveWorkspaceMember(ctx, wsID, targetUserID)
}

// isAssignableRole は招待 / role 変更で指定可能な role か。
// owner は所有者 (account 経由) に予約されており、明示付与は許可しない。
func isAssignableRole(role domain.WorkspaceRole) bool {
	return role == domain.WorkspaceRoleEditor || role == domain.WorkspaceRoleViewer
}

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

func (s *WorkspaceService) ListShareLinks(ctx context.Context, wsID, userID string) ([]*domain.ShareLink, error) {
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return nil, err
	}
	return s.workspaces.ListShareLinks(ctx, wsID)
}

func (s *WorkspaceService) RevokeShareLink(ctx context.Context, wsID, userID, token string) error {
	if err := s.requireManageMembers(ctx, wsID, userID); err != nil {
		return err
	}
	return s.workspaces.RevokeShareLink(ctx, wsID, token, time.Now().UTC())
}

// ResolveShareLink は無認証で呼ばれる。有効な token のみ workspace と role を返す。
// 認可は token の有効性 (未失効・未期限切れ) のみで、user の権限は問わない。
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

// generateShareToken は推測困難な公開リンク token を生成する (256bit, base64url)。
func generateShareToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
