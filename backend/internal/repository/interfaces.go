package repository

import "github.com/synthify/backend/internal/domain"

type WorkspaceRepository interface {
	ListWorkspaces() []*domain.Workspace
	GetWorkspace(id string) (*domain.Workspace, []*domain.WorkspaceMember, bool)
	CreateWorkspace(name string) *domain.Workspace
	InviteMember(wsID, email, role string, isDev bool) (*domain.WorkspaceMember, bool)
	UpdateMemberRole(wsID, userID, role string, isDev bool) (*domain.WorkspaceMember, bool)
	RemoveMember(wsID, userID string) bool
	TransferOwnership(wsID, newOwnerUserID string) (*domain.Workspace, []*domain.WorkspaceMember, bool)
}

type DocumentRepository interface {
	ListDocuments(wsID string) []*domain.Document
	GetDocument(id string) (*domain.Document, bool)
	CreateDocument(wsID, filename, mimeType string, fileSize int64) (*domain.Document, string)
	GetUploadURL(wsID, filename, mimeType string, fileSize int64) (string, string)
	StartProcessing(docID string, forceReprocess bool, depth string) (*domain.Document, bool)
}

type GraphRepository interface {
	GetGraph(docID string) ([]*domain.Node, []*domain.Edge, bool)
	FindPaths(docID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool)
}

type NodeRepository interface {
	GetNode(nodeID string) (*domain.Node, []*domain.Edge, bool)
	CreateNode(docID, label, category, description, parentNodeID string, level int, createdBy string) *domain.Node
	RecordView(userID, wsID, nodeID, docID string)
	GetUserNodeActivity(wsID, userID, documentID string, limit int) domain.UserNodeActivity
	ApproveAlias(wsID, canonicalNodeID, aliasNodeID string) bool
	RejectAlias(wsID, canonicalNodeID, aliasNodeID string) bool
}
