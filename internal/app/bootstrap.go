package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Keyhole-Koro/SynthifyShared/domain"
	"github.com/Keyhole-Koro/SynthifyShared/repository"
	"github.com/synthify/backend/internal/repository/mock"
	"github.com/synthify/backend/internal/repository/postgres"
)

type Store interface {
	GetOrCreateAccount(userID string) (*domain.Account, error)
	GetAccount(accountID string) (*domain.Account, error)
	ListWorkspacesByUser(userID string) []*domain.Workspace
	GetWorkspace(id string) (*domain.Workspace, bool)
	IsWorkspaceAccessible(wsID, userID string) bool
	CreateWorkspace(accountID, name string) *domain.Workspace
	ListDocuments(wsID string) []*domain.Document
	GetDocument(id string) (*domain.Document, bool)
	CreateDocument(wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string)
	GetLatestProcessingJob(docID string) (*domain.DocumentProcessingJob, bool)
	CreateProcessingJob(docID, graphID, jobType string) *domain.DocumentProcessingJob
	MarkProcessingJobRunning(jobID string) bool
	UpdateProcessingJobStage(jobID, stage string) bool
	FailProcessingJob(jobID, errorMessage string) bool
	CompleteProcessingJob(jobID string) bool
	SaveDocumentChunks(documentID string, chunks []*domain.DocumentChunk) error
	GetOrCreateGraph(wsID string) (*domain.Graph, error)
	GetGraphByWorkspace(wsID string) ([]*domain.Node, []*domain.Edge, bool)
	FindPaths(graphID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool)
	GetNode(nodeID string) (*domain.Node, []*domain.Edge, bool)
	CreateNode(graphID, label, description, parentNodeID, createdBy string) *domain.Node
	CreateStructuredNode(graphID, label, category string, level int, entityType, description, summaryHTML, createdBy string) *domain.Node
	CreateEdge(graphID, sourceNodeID, targetNodeID, edgeType, description string) *domain.Edge
	UpsertNodeSource(nodeID, documentID, chunkID, sourceText string, confidence float64) error
	UpsertEdgeSource(edgeID, documentID, chunkID, sourceText string, confidence float64) error
	UpdateNodeSummaryHTML(nodeID, summaryHTML string) bool
	ApproveAlias(wsID, canonicalNodeID, aliasNodeID string) bool
	RejectAlias(wsID, canonicalNodeID, aliasNodeID string) bool
}

func PublicUploadURLGenerator(base string) repository.UploadURLGenerator {
	return func(workspaceID, documentID string) string {
		return fmt.Sprintf("%s/%s/%s", base, workspaceID, documentID)
	}
}

func InitStore(ctx context.Context, urlGenerator repository.UploadURLGenerator) Store {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		var lastErr error
		for attempt := 1; attempt <= 10; attempt++ {
			store, err := postgres.NewStore(ctx, dsn, urlGenerator)
			if err == nil {
				log.Printf("using postgres store")
				return store
			}
			lastErr = err
			log.Printf("failed to connect postgres (attempt %d/10): %v", attempt, err)
			time.Sleep(2 * time.Second)
		}
		log.Fatalf("failed to connect postgres after retries: %v", lastErr)
	}
	log.Printf("DATABASE_URL is empty, falling back to mock store")
	return mock.NewStore(urlGenerator)
}
