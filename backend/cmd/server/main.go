package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/synthify/backend/internal/handler"
	"github.com/synthify/backend/internal/middleware"
	"github.com/synthify/backend/internal/repository/mock"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:5173"
	}

	store := mock.NewStore()

	wh := handler.NewWorkspaceHandler(store)
	dh := handler.NewDocumentHandler(store)
	gh := handler.NewGraphHandler(store)
	nh := handler.NewNodeHandler(store)

	mux := http.NewServeMux()

	// WorkspaceService
	mux.HandleFunc("POST /synthify.graph.v1.WorkspaceService/ListWorkspaces", wh.ListWorkspaces)
	mux.HandleFunc("POST /synthify.graph.v1.WorkspaceService/GetWorkspace", wh.GetWorkspace)
	mux.HandleFunc("POST /synthify.graph.v1.WorkspaceService/CreateWorkspace", wh.CreateWorkspace)
	mux.HandleFunc("POST /synthify.graph.v1.WorkspaceService/InviteMember", wh.InviteMember)

	// DocumentService
	mux.HandleFunc("POST /synthify.graph.v1.DocumentService/ListDocuments", dh.ListDocuments)
	mux.HandleFunc("POST /synthify.graph.v1.DocumentService/GetDocument", dh.GetDocument)
	mux.HandleFunc("POST /synthify.graph.v1.DocumentService/CreateDocument", dh.CreateDocument)
	mux.HandleFunc("POST /synthify.graph.v1.DocumentService/StartProcessing", dh.StartProcessing)
	mux.HandleFunc("POST /synthify.graph.v1.DocumentService/ResumeProcessing", dh.ResumeProcessing)

	// GraphService
	mux.HandleFunc("POST /synthify.graph.v1.GraphService/GetGraph", gh.GetGraph)

	// NodeService
	mux.HandleFunc("POST /synthify.graph.v1.NodeService/GetGraphEntityDetail", nh.GetGraphEntityDetail)
	mux.HandleFunc("POST /synthify.graph.v1.NodeService/RecordNodeView", nh.RecordNodeView)
	mux.HandleFunc("POST /synthify.graph.v1.NodeService/CreateNode", nh.CreateNode)

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
