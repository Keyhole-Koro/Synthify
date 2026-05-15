package mappers

import (
	"github.com/synthify/backend/packages/shared/document"
	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

func ToProtoDocument(doc *domain.Document, latestJob *domain.DocumentProcessingJob) *treev1.Document {
	if doc == nil {
		return nil
	}
	return &treev1.Document{
		DocumentId:  doc.DocumentID,
		WorkspaceId: doc.WorkspaceID,
		UploadedBy:  doc.UploadedBy,
		Filename:    doc.Filename,
		MimeType:    doc.MimeType,
		FileSize:    doc.FileSize,
		Status:      document.DeriveLifecycleState(latestJob),
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.CreatedAt,
	}
}
