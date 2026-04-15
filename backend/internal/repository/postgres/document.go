package postgres

import (
	"context"
	"database/sql"
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

func (s *Store) CreateDocument(wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string) {
	createdAt := nowTime()
	doc := &domain.Document{
		DocumentID:  newID(),
		WorkspaceID: wsID,
		UploadedBy:  uploadedBy,
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    fileSize,
		CreatedAt:   createdAt.Format(time.RFC3339),
	}
	err := s.q().CreateDocument(context.Background(), sqlcgen.CreateDocumentParams{
		DocumentID:  doc.DocumentID,
		WorkspaceID: doc.WorkspaceID,
		UploadedBy:  doc.UploadedBy,
		Filename:    doc.Filename,
		MimeType:    doc.MimeType,
		FileSize:    doc.FileSize,
		CreatedAt:   createdAt,
	})
	if err != nil {
		return nil, ""
	}
	return doc, s.uploadURLGenerator(wsID, doc.DocumentID)
}

func (s *Store) GetLatestProcessingJob(docID string) (*domain.DocumentProcessingJob, bool) {
	row, err := s.q().GetLatestProcessingJob(context.Background(), docID)
	if err != nil {
		return nil, false
	}
	return toProcessingJob(sqlcgen.DocumentProcessingJob{
		JobID:        row.JobID,
		DocumentID:   row.DocumentID,
		GraphID:      sql.NullString{String: row.GraphID, Valid: row.GraphID != ""},
		JobType:      row.JobType,
		Status:       row.Status,
		CurrentStage: row.CurrentStage,
		ErrorMessage: row.ErrorMessage,
		ParamsJson:   row.ParamsJson,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}), true
}

func (s *Store) CreateProcessingJob(docID, graphID, jobType string) *domain.DocumentProcessingJob {
	createdAt := nowTime()
	jobID := newID()
	if err := s.q().CreateProcessingJob(context.Background(), sqlcgen.CreateProcessingJobParams{
		JobID:      jobID,
		DocumentID: docID,
		Column3:    graphID,
		JobType:    jobType,
		Status:     "queued",
		CreatedAt:  createdAt,
	}); err != nil {
		return nil
	}
	return &domain.DocumentProcessingJob{
		JobID:      jobID,
		DocumentID: docID,
		GraphID:    graphID,
		JobType:    jobType,
		Status:     "queued",
		CreatedAt:  createdAt.Format(time.RFC3339),
		UpdatedAt:  createdAt.Format(time.RFC3339),
	}
}

func (s *Store) CompleteProcessingJob(jobID string) bool {
	affected, err := s.q().CompleteProcessingJob(context.Background(), sqlcgen.CompleteProcessingJobParams{
		JobID:     jobID,
		UpdatedAt: nowTime(),
	})
	return err == nil && affected > 0
}
