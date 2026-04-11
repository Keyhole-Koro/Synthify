package postgres

import (
	"time"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

func toWorkspace(row sqlcgen.Workspace) *domain.Workspace {
	return &domain.Workspace{
		WorkspaceID:       row.WorkspaceID,
		Name:              row.Name,
		OwnerID:           row.OwnerID,
		Plan:              row.Plan,
		StorageUsedBytes:  row.StorageUsedBytes,
		StorageQuotaBytes: row.StorageQuotaBytes,
		MaxFileSizeBytes:  row.MaxFileSizeBytes,
		MaxUploadsPerDay:  row.MaxUploadsPerDay,
		CreatedAt:         row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toWorkspaceMember(row sqlcgen.ListWorkspaceMembersRow) *domain.WorkspaceMember {
	return &domain.WorkspaceMember{
		UserID:    row.UserID,
		Email:     row.Email,
		Role:      domain.WorkspaceRole(row.Role),
		IsDev:     row.IsDev,
		InvitedAt: row.InvitedAt.UTC().Format(time.RFC3339),
		InvitedBy: row.InvitedBy,
	}
}

func toWorkspaceMemberFromGet(row sqlcgen.GetWorkspaceMemberRow) *domain.WorkspaceMember {
	return &domain.WorkspaceMember{
		UserID:    row.UserID,
		Email:     row.Email,
		Role:      domain.WorkspaceRole(row.Role),
		IsDev:     row.IsDev,
		InvitedAt: row.InvitedAt.UTC().Format(time.RFC3339),
		InvitedBy: row.InvitedBy,
	}
}

func toDocument(row sqlcgen.Document) *domain.Document {
	return &domain.Document{
		DocumentID:      row.DocumentID,
		WorkspaceID:     row.WorkspaceID,
		UploadedBy:      row.UploadedBy,
		Filename:        row.Filename,
		MimeType:        row.MimeType,
		FileSize:        row.FileSize,
		Status:          domain.DocumentLifecycleState(row.Status),
		ExtractionDepth: row.ExtractionDepth,
		NodeCount:       int(row.NodeCount),
		CurrentStage:    row.CurrentStage,
		ErrorMessage:    row.ErrorMessage,
		CreatedAt:       row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toNode(row sqlcgen.Node) *domain.Node {
	return &domain.Node{
		NodeID:      row.NodeID,
		DocumentID:  row.DocumentID,
		Label:       row.Label,
		Level:       int(row.Level),
		Category:    row.Category,
		EntityType:  row.EntityType,
		Description: row.Description,
		SummaryHTML: row.SummaryHtml,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toEdge(row sqlcgen.Edge) *domain.Edge {
	return &domain.Edge{
		EdgeID:       row.EdgeID,
		DocumentID:   row.DocumentID,
		SourceNodeID: row.SourceNodeID,
		TargetNodeID: row.TargetNodeID,
		EdgeType:     row.EdgeType,
		Description:  row.Description,
		CreatedAt:    row.CreatedAt.UTC().Format(time.RFC3339),
	}
}
