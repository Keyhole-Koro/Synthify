package storage

import (
	"context"

	"github.com/synthify/backend/apps/api/internal/repository"
	platformstorage "github.com/synthify/backend/internal/platform/storage"
)

type fakeGCSDocumentUploadURLIssuer struct {
	base   string
	bucket string
}

// NewFakeGCSDocumentUploadURLIssuer creates the local fake-gcs implementation
// of the document upload port.
func NewFakeGCSDocumentUploadURLIssuer(base, bucket string) repository.DocumentUploadURLIssuer {
	return fakeGCSDocumentUploadURLIssuer{base: base, bucket: bucket}
}

func (i fakeGCSDocumentUploadURLIssuer) IssueDocumentUploadURL(_ context.Context, workspaceID, objectName, contentType string, fileSize int64) (repository.DocumentUploadTarget, error) {
	_ = fileSize
	return repository.DocumentUploadTarget{
		URL:         platformstorage.BuildDocumentUploadURL(i.base, i.bucket, workspaceID, objectName),
		Method:      "POST",
		ContentType: contentType,
	}, nil
}
