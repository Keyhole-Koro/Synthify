package service

import (
	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository"
)

type DocumentService struct {
	repo repository.DocumentRepository
}

func NewDocumentService(repo repository.DocumentRepository) *DocumentService {
	return &DocumentService{repo: repo}
}

func (s *DocumentService) ListDocuments(workspaceID string) []*domain.Document {
	return s.repo.ListDocuments(workspaceID)
}

func (s *DocumentService) GetDocument(documentID string) (*domain.Document, error) {
	doc, ok := s.repo.GetDocument(documentID)
	if !ok {
		return nil, ErrNotFound
	}
	return doc, nil
}

func (s *DocumentService) CreateDocument(wsID, filename, mimeType string, fileSize int64) (*domain.Document, string) {
	return s.repo.CreateDocument(wsID, filename, mimeType, fileSize)
}

func (s *DocumentService) GetUploadURL(wsID, filename, mimeType string, fileSize int64) (string, string) {
	return s.repo.GetUploadURL(wsID, filename, mimeType, fileSize)
}

func (s *DocumentService) StartProcessing(documentID string, forceReprocess bool, extractionDepth string) (*domain.Document, error) {
	doc, ok := s.repo.StartProcessing(documentID, forceReprocess, extractionDepth)
	if !ok {
		return nil, ErrNotFound
	}
	return doc, nil
}
