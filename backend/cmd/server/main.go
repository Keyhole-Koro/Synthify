package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	graphv1connect "github.com/synthify/backend/gen/synthify/graph/v1/graphv1connect"
	"github.com/synthify/backend/internal/handler"
	"github.com/synthify/backend/internal/middleware"
	"github.com/synthify/backend/internal/repository/mock"
	"github.com/synthify/backend/internal/service"
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
