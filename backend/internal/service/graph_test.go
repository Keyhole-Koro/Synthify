package service

import (
	"errors"
	"testing"

	"github.com/synthify/backend/internal/repository/mock"
)

func setupGraphFixtures(t *testing.T) (*mock.Store, string) {
	t.Helper()
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "u1", "u@example.com")
	doc, _ := store.CreateDocument(ws.WorkspaceID, "test.pdf", "application/pdf", 1024)
	store.StartProcessing(doc.DocumentID, false, "full")
	return store, doc.DocumentID
}

func TestGetGraph_DocumentNotFound_ReturnsErrNotFound(t *testing.T) {
	store := mock.NewStore()
	svc := NewGraphService(store)

	_, _, err := svc.GetGraph("nonexistent_doc")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetGraph missing doc: err = %v, want ErrNotFound", err)
	}
}

func TestGetGraph_ProcessedDocument_ReturnsNodesAndEdges(t *testing.T) {
	store, docID := setupGraphFixtures(t)
	svc := NewGraphService(store)

	nodes, edges, err := svc.GetGraph(docID)
	if err != nil {
		t.Fatalf("GetGraph: unexpected error: %v", err)
	}
	if len(nodes) == 0 {
		t.Error("expected nodes, got none")
	}
	if len(edges) == 0 {
		t.Error("expected edges, got none")
	}
}

func TestFindPaths_DocumentNotFound_ReturnsErrNotFound(t *testing.T) {
	store := mock.NewStore()
	svc := NewGraphService(store)

	_, _, _, err := svc.FindPaths("nonexistent_doc", "n1", "n2", 4, 3)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("FindPaths missing doc: err = %v, want ErrNotFound", err)
	}
}

func TestFindPaths_ConnectedNodes_ReturnsPaths(t *testing.T) {
	store, docID := setupGraphFixtures(t)
	svc := NewGraphService(store)

	// nd_root → nd_tel → nd_cv is a known path in the seed data.
	nodes, edges, paths, err := svc.FindPaths(docID, "nd_root", "nd_cv", 4, 3)
	if err != nil {
		t.Fatalf("FindPaths: unexpected error: %v", err)
	}
	if len(nodes) == 0 {
		t.Error("expected nodes in result")
	}
	if len(edges) == 0 {
		t.Error("expected edges in result")
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one path from nd_root to nd_cv")
	}
	if paths[0].NodeIDs[0] != "nd_root" {
		t.Errorf("path start = %q, want nd_root", paths[0].NodeIDs[0])
	}
	last := paths[0].NodeIDs[len(paths[0].NodeIDs)-1]
	if last != "nd_cv" {
		t.Errorf("path end = %q, want nd_cv", last)
	}
}
