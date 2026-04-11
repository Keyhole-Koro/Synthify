package postgres

import (
	"fmt"

	"github.com/synthify/backend/internal/domain"
)

func (s *Store) ListDocuments(wsID string) []*domain.Document {
	rows, err := s.db.Query(`
		SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, status, extraction_depth, node_count, current_stage, error_message, created_at, updated_at
		FROM documents
		WHERE workspace_id = $1
		ORDER BY created_at DESC
	`, wsID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var docs []*domain.Document
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err == nil {
			docs = append(docs, doc)
		}
	}
	return docs
}

func (s *Store) GetDocument(id string) (*domain.Document, bool) {
	row := s.db.QueryRow(`
		SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, status, extraction_depth, node_count, current_stage, error_message, created_at, updated_at
		FROM documents
		WHERE document_id = $1
	`, id)
	doc, err := scanDocument(row)
	return doc, err == nil
}

func (s *Store) CreateDocument(wsID, filename, mimeType string, fileSize int64) (*domain.Document, string) {
	doc := &domain.Document{
		DocumentID:  newID("doc"),
		WorkspaceID: wsID,
		UploadedBy:  "user_demo",
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    fileSize,
		Status:      domain.DocumentLifecycleUploaded,
		CreatedAt:   now(),
		UpdatedAt:   now(),
	}
	_, err := s.db.Exec(`
		INSERT INTO documents (document_id, workspace_id, uploaded_by, filename, mime_type, file_size, status, extraction_depth, node_count, current_stage, error_message, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, doc.DocumentID, doc.WorkspaceID, doc.UploadedBy, doc.Filename, doc.MimeType, doc.FileSize, doc.Status, doc.ExtractionDepth, doc.NodeCount, doc.CurrentStage, doc.ErrorMessage, doc.CreatedAt, doc.UpdatedAt)
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
	var nodeCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE document_id = $1`, docID).Scan(&nodeCount)
	_, err := s.db.Exec(`
		UPDATE documents
		SET status = 'completed', extraction_depth = $2, current_stage = '', node_count = $3, updated_at = $4
		WHERE document_id = $1
	`, docID, depth, nodeCount, now())
	if err != nil {
		return nil, false
	}
	return s.GetDocument(docID)
}
