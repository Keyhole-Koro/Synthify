package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

func (s *Store) ListDocuments(wsID string) []*domain.Document {
	rows, err := s.q().ListDocuments(context.Background(), wsID)
	if err != nil {
		return nil
	}

	var docs []*domain.Document
	for _, row := range rows {
		docs = append(docs, toDocument(row))
	}
	return docs
}

func (s *Store) GetDocument(id string) (*domain.Document, bool) {
	row, err := s.q().GetDocument(context.Background(), id)
	if err != nil {
		return nil, false
	}
	return toDocument(row), true
}

func (s *Store) CreateDocument(wsID, filename, mimeType string, fileSize int64) (*domain.Document, string) {
	createdAt := nowTime()
	doc := &domain.Document{
		DocumentID:  newID("doc"),
		WorkspaceID: wsID,
		UploadedBy:  "user_demo",
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    fileSize,
		Status:      domain.DocumentLifecycleUploaded,
		CreatedAt:   createdAt.Format(time.RFC3339),
		UpdatedAt:   createdAt.Format(time.RFC3339),
	}
	err := s.q().CreateDocument(context.Background(), sqlcgen.CreateDocumentParams{
		DocumentID:      doc.DocumentID,
		WorkspaceID:     doc.WorkspaceID,
		UploadedBy:      doc.UploadedBy,
		Filename:        doc.Filename,
		MimeType:        doc.MimeType,
		FileSize:        doc.FileSize,
		Status:          string(doc.Status),
		ExtractionDepth: doc.ExtractionDepth,
		NodeCount:       int32(doc.NodeCount),
		CurrentStage:    doc.CurrentStage,
		ErrorMessage:    doc.ErrorMessage,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	})
	if err != nil {
		return nil, ""
	}
	return doc, fmt.Sprintf("http://gcs:4443/synthify-uploads/%s/%s", wsID, doc.DocumentID)
}

func (s *Store) GetUploadURL(wsID, filename, mimeType string, fileSize int64) (string, string) {
	token := newID("upload")
	return fmt.Sprintf("http://gcs:4443/synthify-uploads/%s/%s/%s", wsID, token, filename), token
}

func (s *Store) StartProcessing(docID string, forceReprocess bool, depth string) (*domain.Document, bool) {
	if depth == "" {
		depth = "full"
	}
	ctx := context.Background()
	nodeCount, err := s.q().CountNodesByDocument(ctx, docID)
	if err != nil {
		return nil, false
	}
	updatedAt := nowTime()
	affected, err := s.q().CompleteDocumentProcessing(ctx, sqlcgen.CompleteDocumentProcessingParams{
		DocumentID:      docID,
		ExtractionDepth: depth,
		NodeCount:       int32(nodeCount),
		UpdatedAt:       updatedAt,
	})
	if err != nil {
		return nil, false
	}
	if affected == 0 {
		return nil, false
	}
	return s.GetDocument(docID)
}
