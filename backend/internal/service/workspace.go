package service

import (
	"errors"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository"
)

var ErrNotFound = errors.New("not found")

type WorkspaceService struct {
	repo repository.WorkspaceRepository
}

func NewWorkspaceService(repo repository.WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{repo: repo}
}

func (s *WorkspaceService) ListWorkspaces(userID string) []*domain.Workspace {
	return s.repo.ListWorkspacesByUser(userID)
}

func (s *WorkspaceService) GetWorkspace(id, userID string) (*domain.Workspace, []*domain.WorkspaceMember, error) {
	if !s.repo.IsWorkspaceMember(id, userID) {
		return nil, nil, ErrNotFound
	}
	ws, members, ok := s.repo.GetWorkspace(id)
	if !ok {
		return nil, nil, ErrNotFound
	}
	return ws, members, nil
}

func (s *WorkspaceService) CreateWorkspace(name, ownerUserID, ownerEmail string) *domain.Workspace {
	return s.repo.CreateWorkspace(name, ownerUserID, ownerEmail)
}

func (s *WorkspaceService) InviteMember(wsID, email, role string, isDev bool) (*domain.WorkspaceMember, error) {
	member, ok := s.repo.InviteMember(wsID, email, role, isDev)
	if !ok {
		return nil, ErrNotFound
	}
	return member, nil
}

func (s *WorkspaceService) UpdateMemberRole(wsID, userID, role string, isDev bool) (*domain.WorkspaceMember, error) {
	member, ok := s.repo.UpdateMemberRole(wsID, userID, role, isDev)
	if !ok {
		return nil, ErrNotFound
	}
	return member, nil
}

func (s *WorkspaceService) RemoveMember(wsID, userID string) error {
	if !s.repo.RemoveMember(wsID, userID) {
		return ErrNotFound
	}
	return nil
}

func (s *WorkspaceService) TransferOwnership(wsID, newOwnerUserID string) (*domain.Workspace, []*domain.WorkspaceMember, error) {
	ws, members, ok := s.repo.TransferOwnership(wsID, newOwnerUserID)
	if !ok {
		return nil, nil, ErrNotFound
	}
	return ws, members, nil
}
