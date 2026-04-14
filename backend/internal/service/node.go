package service

import (
	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository"
)

type NodeService struct {
	repo repository.NodeRepository
}

func NewNodeService(repo repository.NodeRepository) *NodeService {
	return &NodeService{repo: repo}
}

func (s *NodeService) GetGraphEntityDetail(nodeID string) (*domain.Node, []*domain.Edge, error) {
	node, edges, ok := s.repo.GetNode(nodeID)
	if !ok {
		return nil, nil, ErrNotFound
	}
	return node, edges, nil
}

func (s *NodeService) RecordNodeView(userID, workspaceID, nodeID, documentID string) {
	s.repo.RecordView(userID, workspaceID, nodeID, documentID)
}

func (s *NodeService) CreateNode(userID, documentID, label, category, description, parentNodeID string, level int) *domain.Node {
	return s.repo.CreateNode(documentID, label, category, description, parentNodeID, level, userID)
}

func (s *NodeService) GetUserNodeActivity(workspaceID, userID, documentID string, limit int) domain.UserNodeActivity {
	return s.repo.GetUserNodeActivity(workspaceID, userID, documentID, limit)
}

func (s *NodeService) ApproveAlias(workspaceID, canonicalNodeID, aliasNodeID string) error {
	if !s.repo.ApproveAlias(workspaceID, canonicalNodeID, aliasNodeID) {
		return ErrNotFound
	}
	return nil
}

func (s *NodeService) RejectAlias(workspaceID, canonicalNodeID, aliasNodeID string) error {
	if !s.repo.RejectAlias(workspaceID, canonicalNodeID, aliasNodeID) {
		return ErrNotFound
	}
	return nil
}
