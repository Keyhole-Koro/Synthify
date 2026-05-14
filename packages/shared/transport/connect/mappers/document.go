package mappers

import (
	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

func ToProtoDocument(doc *domain.Document) *treev1.Document {
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
		Status:      treev1.DocumentLifecycleState_DOCUMENT_LIFECYCLE_STATE_UPLOADED,
		CreatedAt:   doc.CreatedAt,
		UpdatedAt:   doc.CreatedAt,
	}
}
