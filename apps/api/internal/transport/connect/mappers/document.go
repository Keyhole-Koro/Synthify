package mappers

import (
	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func ToProtoDocument(doc *domain.Document, latestJob *domain.DocumentProcessingJob) *appv1.Document {
	if doc == nil {
		return nil
	}
	return &appv1.Document{
		DocumentId:  doc.DocumentID,
		WorkspaceId: doc.WorkspaceID,
		UploadedBy:  doc.UploadedBy,
		Filename:    doc.Filename,
		MimeType:    doc.MimeType,
		FileSize:    doc.FileSize,
		Status:      domain.DeriveLifecycleState(latestJob),
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.CreatedAt,
	}
}
