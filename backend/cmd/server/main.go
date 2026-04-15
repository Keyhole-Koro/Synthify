package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	graphv1connect "github.com/synthify/backend/gen/synthify/graph/v1/graphv1connect"
	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/handler"
	"github.com/synthify/backend/internal/middleware"
	"github.com/synthify/backend/internal/repository"
	"github.com/synthify/backend/internal/repository/mock"
	"github.com/synthify/backend/internal/repository/postgres"
	"github.com/synthify/backend/internal/service"
)

func main() {
	ctx := context.Background()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:5173"
	}

	uploadURLBase := os.Getenv("GCS_UPLOAD_URL_BASE")
	if uploadURLBase == "" {
		uploadURLBase = "http://localhost:4443/synthify-uploads"
	}

	// URL generation logic.
	urlGenerator := func(workspaceID, documentID string) string {
		return fmt.Sprintf("%s/%s/%s", uploadURLBase, workspaceID, documentID)
	}

	store := initStore(ctx, urlGenerator)

	workspaceService := service.NewWorkspaceService(store, store)
	documentService := service.NewDocumentService(store, store)
	graphService := service.NewGraphService(store)
	nodeService := service.NewNodeService(store, store)

	wh := handler.NewWorkspaceHandler(workspaceService)
	dh := handler.NewDocumentHandler(documentService, store, store, urlGenerator)
	gh := handler.NewGraphHandler(graphService, store, store)
	nh := handler.NewNodeHandler(nodeService, store, store)

	mux := http.NewServeMux()

	mux.Handle(graphv1connect.NewWorkspaceServiceHandler(wh))
	mux.Handle(graphv1connect.NewDocumentServiceHandler(dh))
	mux.Handle(graphv1connect.NewGraphServiceHandler(gh))
	mux.Handle(graphv1connect.NewNodeServiceHandler(nh))

	// Health check.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	h := middleware.Logger(middleware.CORS(corsOrigins, middleware.WithAuth(os.Getenv("FIREBASE_PROJECT_ID"), mux)))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Synthify backend listening on %s (CORS: %s)", addr, corsOrigins)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}

type appStore interface {
	// AccountRepository
	GetOrCreateAccount(userID string) (*domain.Account, error)
	GetAccount(accountID string) (*domain.Account, error)
	// WorkspaceRepository
	ListWorkspacesByUser(userID string) []*domain.Workspace
	GetWorkspace(id string) (*domain.Workspace, bool)
	IsWorkspaceAccessible(wsID, userID string) bool
	CreateWorkspace(accountID, name string) *domain.Workspace
	// DocumentRepository
	ListDocuments(wsID string) []*domain.Document
	GetDocument(id string) (*domain.Document, bool)
	CreateDocument(wsID, uploadedBy, filename, mimeType string, fileSize int64) (*domain.Document, string)
	GetLatestProcessingJob(docID string) (*domain.DocumentProcessingJob, bool)
	CreateProcessingJob(docID, graphID, jobType string) *domain.DocumentProcessingJob
	CompleteProcessingJob(jobID string) bool
	// GraphRepository
	GetOrCreateGraph(wsID string) (*domain.Graph, error)
	GetGraphByWorkspace(wsID string) ([]*domain.Node, []*domain.Edge, bool)
	FindPaths(graphID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool)
	// NodeRepository
	GetNode(nodeID string) (*domain.Node, []*domain.Edge, bool)
	CreateNode(graphID, label, description, parentNodeID, createdBy string) *domain.Node
	UpsertNodeSource(nodeID, documentID, chunkID, sourceText string, confidence float64) error
	ApproveAlias(wsID, canonicalNodeID, aliasNodeID string) bool
	RejectAlias(wsID, canonicalNodeID, aliasNodeID string) bool
}

func initStore(ctx context.Context, urlGenerator repository.UploadURLGenerator) appStore {
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
