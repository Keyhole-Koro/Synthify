package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	graphv1connect "github.com/synthify/backend/gen/synthify/graph/v1/graphv1connect"
	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/handler"
	"github.com/synthify/backend/internal/middleware"
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

	store := initStore(ctx)

	workspaceService := service.NewWorkspaceService(store)
	documentService := service.NewDocumentService(store)
	graphService := service.NewGraphService(store)
	nodeService := service.NewNodeService(store)

	wh := handler.NewWorkspaceHandler(workspaceService)
	dh := handler.NewDocumentHandler(documentService)
	gh := handler.NewGraphHandler(graphService)
	nh := handler.NewNodeHandler(nodeService)

	mux := http.NewServeMux()

	mux.Handle(graphv1connect.NewWorkspaceServiceHandler(wh))
	mux.Handle(graphv1connect.NewDocumentServiceHandler(dh))
	mux.Handle(graphv1connect.NewGraphServiceHandler(gh))
	mux.Handle(graphv1connect.NewNodeServiceHandler(nh))

	// ヘルスチェック
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	h := middleware.Logger(middleware.CORS(corsOrigins, mux))

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Synthify backend listening on %s (CORS: %s)", addr, corsOrigins)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}

type appStore interface {
	ListWorkspaces() []*domain.Workspace
	GetWorkspace(id string) (*domain.Workspace, []*domain.WorkspaceMember, bool)
	CreateWorkspace(name string) *domain.Workspace
	InviteMember(wsID, email, role string, isDev bool) (*domain.WorkspaceMember, bool)
	UpdateMemberRole(wsID, userID, role string, isDev bool) (*domain.WorkspaceMember, bool)
	RemoveMember(wsID, userID string) bool
	TransferOwnership(wsID, newOwnerUserID string) (*domain.Workspace, []*domain.WorkspaceMember, bool)
	ListDocuments(wsID string) []*domain.Document
	GetDocument(id string) (*domain.Document, bool)
	CreateDocument(wsID, filename, mimeType string, fileSize int64) (*domain.Document, string)
	GetUploadURL(wsID, filename, mimeType string, fileSize int64) (string, string)
	StartProcessing(docID string, forceReprocess bool, depth string) (*domain.Document, bool)
	GetGraph(docID string) ([]*domain.Node, []*domain.Edge, bool)
	FindPaths(docID, sourceNodeID, targetNodeID string, maxDepth, limit int) ([]*domain.Node, []*domain.Edge, []domain.GraphPath, bool)
	GetNode(nodeID string) (*domain.Node, []*domain.Edge, bool)
	CreateNode(docID, label, category, description, parentNodeID string, level int, createdBy string) *domain.Node
	RecordView(userID, wsID, nodeID, docID string)
	GetUserNodeActivity(wsID, userID, documentID string, limit int) domain.UserNodeActivity
	ApproveAlias(wsID, canonicalNodeID, aliasNodeID string) bool
	RejectAlias(wsID, canonicalNodeID, aliasNodeID string) bool
}

func initStore(ctx context.Context) appStore {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		store, err := postgres.NewStore(ctx, dsn)
		if err != nil {
			log.Fatalf("failed to connect postgres: %v", err)
		}
		log.Printf("using postgres store")
		return store
	}
	log.Printf("DATABASE_URL is empty, falling back to mock store")
	return mock.NewStore()
}
