package postgres

import (
	"time"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

func toAccount(row sqlcgen.Account) *domain.Account {
	return &domain.Account{
		AccountID:          row.AccountID,
		Name:               row.Name,
		Plan:               row.Plan,
		StorageQuotaBytes:  row.StorageQuotaBytes,
		StorageUsedBytes:   row.StorageUsedBytes,
		MaxFileSizeBytes:   row.MaxFileSizeBytes,
		MaxUploadsPerFiveH: row.MaxUploadsPer5h,
		MaxUploadsPerWeek:  row.MaxUploadsPer1week,
		CreatedAt:          row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toWorkspace(row sqlcgen.Workspace) *domain.Workspace {
	return &domain.Workspace{
		WorkspaceID: row.WorkspaceID,
		AccountID:   row.AccountID,
		Name:        row.Name,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toDocument(row sqlcgen.Document) *domain.Document {
	return &domain.Document{
		DocumentID:  row.DocumentID,
		WorkspaceID: row.WorkspaceID,
		UploadedBy:  row.UploadedBy,
		Filename:    row.Filename,
		MimeType:    row.MimeType,
		FileSize:    row.FileSize,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toProcessingJob(row sqlcgen.DocumentProcessingJob) *domain.DocumentProcessingJob {
	graphID := ""
	if row.GraphID.Valid {
		graphID = row.GraphID.String
	}
	return &domain.DocumentProcessingJob{
		JobID:        row.JobID,
		DocumentID:   row.DocumentID,
		GraphID:      graphID,
		JobType:      row.JobType,
		Status:       row.Status,
		CurrentStage: row.CurrentStage,
		ErrorMessage: row.ErrorMessage,
		ParamsJSON:   row.ParamsJson,
		CreatedAt:    row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toGraph(row sqlcgen.Graph) *domain.Graph {
	return &domain.Graph{
		GraphID:     row.GraphID,
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toNode(row sqlcgen.GetNodeRow) *domain.Node {
	return &domain.Node{
		NodeID:      row.NodeID,
		GraphID:     row.GraphID,
		Label:       row.Label,
		EntityType:  row.EntityType,
		Description: row.Description,
		SummaryHTML: row.SummaryHtml,
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toNodeFromListRow(row sqlcgen.ListNodesByGraphRow) *domain.Node {
	return &domain.Node{
		NodeID:      row.NodeID,
		GraphID:     row.GraphID,
		Label:       row.Label,
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
		GraphID:      row.GraphID,
		SourceNodeID: row.SourceNodeID,
		TargetNodeID: row.TargetNodeID,
		EdgeType:     row.EdgeType,
		Description:  row.Description,
		CreatedAt:    row.CreatedAt.UTC().Format(time.RFC3339),
	}
}
