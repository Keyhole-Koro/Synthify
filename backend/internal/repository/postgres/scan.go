package postgres

import (
	"time"

	"github.com/synthify/backend/internal/domain"
)

func scanWorkspace(row scanner) (*domain.Workspace, error) {
	var workspace domain.Workspace
	var createdAt time.Time
	err := row.Scan(&workspace.WorkspaceID, &workspace.Name, &workspace.OwnerID, &workspace.Plan, &workspace.StorageUsedBytes, &workspace.StorageQuotaBytes, &workspace.MaxFileSizeBytes, &workspace.MaxUploadsPerDay, &createdAt)
	workspace.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &workspace, err
}

func scanWorkspaceMember(row scanner) (*domain.WorkspaceMember, error) {
	var member domain.WorkspaceMember
	var invitedAt time.Time
	err := row.Scan(&member.UserID, &member.Email, &member.Role, &member.IsDev, &invitedAt, &member.InvitedBy)
	member.InvitedAt = invitedAt.UTC().Format(time.RFC3339)
	return &member, err
}

func scanDocument(row scanner) (*domain.Document, error) {
	var document domain.Document
	var createdAt, updatedAt time.Time
	err := row.Scan(&document.DocumentID, &document.WorkspaceID, &document.UploadedBy, &document.Filename, &document.MimeType, &document.FileSize, &document.Status, &document.ExtractionDepth, &document.NodeCount, &document.CurrentStage, &document.ErrorMessage, &createdAt, &updatedAt)
	document.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	document.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return &document, err
}

func scanNode(row scanner) (*domain.Node, error) {
	var node domain.Node
	var createdAt time.Time
	err := row.Scan(&node.NodeID, &node.DocumentID, &node.Label, &node.Level, &node.Category, &node.EntityType, &node.Description, &node.SummaryHTML, &node.CreatedBy, &createdAt)
	node.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &node, err
}

func scanEdge(row scanner) (*domain.Edge, error) {
	var edge domain.Edge
	var createdAt time.Time
	err := row.Scan(&edge.EdgeID, &edge.DocumentID, &edge.SourceNodeID, &edge.TargetNodeID, &edge.EdgeType, &edge.Description, &createdAt)
	edge.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &edge, err
}
