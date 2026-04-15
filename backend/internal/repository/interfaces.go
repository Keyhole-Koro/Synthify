package repository

import "github.com/synthify/backend/internal/domain"

type UploadURLGenerator func(workspaceID, documentID string) string

type AccountRepository interface {
	GetOrCreateAccount(userID string) (*domain.Account, error)
	GetAccount(accountID string) (*domain.Account, error)
}

type WorkspaceRepository interface {
	ListWorkspacesByUser(userID string) []*domain.Workspace
	GetWorkspace(id string) (*domain.Workspace, bool)
	IsWorkspaceAccessible(wsID, userID string) bool
	CreateWorkspace(accountID, name string) *domain.Workspace
}

type DocumentRepository interface {
	ListDocuments(wsID string) []*domain.Document
	GetDocument(id string) (*domain.Document, bool)
	CreateDocument(wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string)
	GetLatestProcessingJob(docID string) (*domain.DocumentProcessingJob, bool)
	CreateProcessingJob(docID, graphID, jobType string) *domain.DocumentProcessingJob
	CompleteProcessingJob(jobID string) bool
}

type GraphRepository interface {
	GetOrCreateGraph(wsID string) (*domain.Graph, error)
	GetGraphByWorkspace(wsID string) ([]*domain.Node, []*domain.Edge, bool)
	FindPaths(graphID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool)
}

type NodeRepository interface {
	GetNode(nodeID string) (*domain.Node, []*domain.Edge, bool)
	CreateNode(graphID, label, description, parentNodeID, createdBy string) *domain.Node
	UpsertNodeSource(nodeID, documentID, chunkID, sourceText string, confidence float64) error
	ApproveAlias(wsID, canonicalNodeID, aliasNodeID string) bool
	RejectAlias(wsID, canonicalNodeID, aliasNodeID string) bool
}
