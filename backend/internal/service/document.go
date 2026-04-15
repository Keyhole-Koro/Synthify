package service

import (
	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository"
)

type DocumentService struct {
	repo  repository.DocumentRepository
	graph repository.GraphRepository
}

func NewDocumentService(repo repository.DocumentRepository, graph repository.GraphRepository) *DocumentService {
	return &DocumentService{repo: repo, graph: graph}
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

func (s *DocumentService) CreateDocument(wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string) {
	return s.repo.CreateDocument(wsID, uploadedBy, filename, mimeType, fileSize)
}

func (s *DocumentService) StartProcessing(wsID, documentID string, forceReprocess bool) (*domain.DocumentProcessingJob, error) {
	if _, ok := s.repo.GetDocument(documentID); !ok {
		return nil, ErrNotFound
	}
	graph, err := s.graph.GetOrCreateGraph(wsID)
	if err != nil {
		return nil, err
	}
	job := s.repo.CreateProcessingJob(documentID, graph.GraphID, "process_document")
	if job == nil {
		return nil, ErrNotFound
	}
	// Mark as completed immediately until the real worker is implemented.
	s.repo.CompleteProcessingJob(job.JobID)
	job.Status = "completed"
	return job, nil
}

func (s *DocumentService) GetLatestProcessingJob(documentID string) (*domain.DocumentProcessingJob, error) {
	job, ok := s.repo.GetLatestProcessingJob(documentID)
	if !ok {
		return nil, ErrNotFound
	}
	return job, nil
}
