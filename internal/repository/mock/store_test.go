package mock

import (
	"testing"

	"github.com/Keyhole-Koro/SynthifyShared/domain"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// setupWorkspace creates an account and workspace for a given userID.
func setupWorkspace(t *testing.T, s *Store, userID string) *domain.Workspace {
	t.Helper()
	acct, err := s.GetOrCreateAccount(userID)
	if err != nil {
		t.Fatalf("GetOrCreateAccount: %v", err)
	}
	ws := s.CreateWorkspace(acct.AccountID, "test-workspace")
	if ws == nil {
		t.Fatal("CreateWorkspace returned nil")
	}
	return ws
}

// setupGraph creates a graph and seed nodes/edges for a workspace.
func setupGraph(t *testing.T, s *Store, wsID string) *domain.Graph {
	t.Helper()
	g, err := s.GetOrCreateGraph(wsID)
	if err != nil {
		t.Fatalf("GetOrCreateGraph: %v", err)
	}
	// Create a processing job to generate seed data.
	doc, _ := s.CreateDocument(wsID, "user1", "f.pdf", "application/pdf", 100)
	s.CreateProcessingJob(doc.DocumentID, g.GraphID, "process_document")
	return g
}

// ── IsWorkspaceAccessible ─────────────────────────────────────────────────────

func TestIsWorkspaceAccessible_Owner_ReturnsTrue(t *testing.T) {
	store := NewStore()
	ws := setupWorkspace(t, store, "owner")

	if !store.IsWorkspaceAccessible(ws.WorkspaceID, "owner") {
		t.Error("owner should have access to their workspace")
	}
}

func TestIsWorkspaceAccessible_Stranger_ReturnsFalse(t *testing.T) {
	store := NewStore()
	ws := setupWorkspace(t, store, "owner")

	if store.IsWorkspaceAccessible(ws.WorkspaceID, "stranger") {
		t.Error("stranger should not have access")
	}
}

// ── ApproveAlias / RejectAlias ────────────────────────────────────────────────

func TestApproveAlias_RecordsAlias(t *testing.T) {
	store := NewStore()
	ws := setupWorkspace(t, store, "u1")
	setupGraph(t, store, ws.WorkspaceID)

	ok := store.ApproveAlias(ws.WorkspaceID, "nd_root", "nd_tel")
	if !ok {
		t.Fatal("ApproveAlias returned false")
	}
	if store.aliases["nd_tel"] != "nd_root" {
		t.Errorf("alias not recorded: aliases[nd_tel] = %q, want nd_root", store.aliases["nd_tel"])
	}
}

func TestApproveAlias_UnknownNode_ReturnsFalse(t *testing.T) {
	store := NewStore()
	ws := setupWorkspace(t, store, "u1")

	ok := store.ApproveAlias(ws.WorkspaceID, "nonexistent", "also_nonexistent")
	if ok {
		t.Error("ApproveAlias unknown nodes: expected false, got true")
	}
}

func TestRejectAlias_RemovesAlias(t *testing.T) {
	store := NewStore()
	ws := setupWorkspace(t, store, "u1")
	setupGraph(t, store, ws.WorkspaceID)

	store.ApproveAlias(ws.WorkspaceID, "nd_root", "nd_tel")

	ok := store.RejectAlias(ws.WorkspaceID, "nd_root", "nd_tel")
	if !ok {
		t.Fatal("RejectAlias returned false")
	}
	if _, found := store.aliases["nd_tel"]; found {
		t.Error("alias still present after rejection")
	}
}

// ── FindPaths (BFS) ───────────────────────────────────────────────────────────

func TestFindPaths_BFS_FindsConnectedPath(t *testing.T) {
	graphID := "g1"
	store := &Store{
		accounts:     make(map[string]*domain.Account),
		accountUsers: make(map[string][]string),
		userAccount:  make(map[string]string),
		workspaces:   make(map[string]*domain.Workspace),
		documents:    make(map[string]*domain.Document),
		jobs:         make(map[string]*domain.DocumentProcessingJob),
		graphs:       map[string]*domain.Graph{"g1": {GraphID: graphID, WorkspaceID: "ws1"}},
		aliases:      make(map[string]string),
		nodes: map[string][]*domain.Node{
			graphID: {
				{NodeID: "n1", GraphID: graphID},
				{NodeID: "n2", GraphID: graphID},
				{NodeID: "n3", GraphID: graphID},
			},
		},
		edges: map[string][]*domain.Edge{
			graphID: {
				{EdgeID: "e1", GraphID: graphID, SourceNodeID: "n1", TargetNodeID: "n2"},
				{EdgeID: "e2", GraphID: graphID, SourceNodeID: "n2", TargetNodeID: "n3"},
			},
		},
	}

	_, _, paths, ok := store.FindPaths(graphID, "n1", "n3", 4, 3)
	if !ok {
		t.Fatal("FindPaths returned false")
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one path")
	}
	if paths[0].NodeIDs[0] != "n1" {
		t.Errorf("path start = %q, want n1", paths[0].NodeIDs[0])
	}
	last := paths[0].NodeIDs[len(paths[0].NodeIDs)-1]
	if last != "n3" {
		t.Errorf("path end = %q, want n3", last)
	}
	if paths[0].HopCount != 2 {
		t.Errorf("hop count = %d, want 2", paths[0].HopCount)
	}
}

func TestFindPaths_NoPathExists_ReturnsEmptyPaths(t *testing.T) {
	graphID := "g1"
	store := &Store{
		accounts:     make(map[string]*domain.Account),
		accountUsers: make(map[string][]string),
		userAccount:  make(map[string]string),
		workspaces:   make(map[string]*domain.Workspace),
		documents:    make(map[string]*domain.Document),
		jobs:         make(map[string]*domain.DocumentProcessingJob),
		graphs:       map[string]*domain.Graph{"g1": {GraphID: graphID, WorkspaceID: "ws1"}},
		aliases:      make(map[string]string),
		nodes: map[string][]*domain.Node{
			graphID: {
				{NodeID: "n1", GraphID: graphID},
				{NodeID: "n2", GraphID: graphID},
				{NodeID: "n3", GraphID: graphID},
			},
		},
		edges: map[string][]*domain.Edge{
			graphID: {
				{EdgeID: "e1", GraphID: graphID, SourceNodeID: "n1", TargetNodeID: "n2"},
			},
		},
	}

	_, _, paths, ok := store.FindPaths(graphID, "n1", "n3", 4, 3)
	if !ok {
		t.Fatal("FindPaths returned false (graph exists)")
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths, got %d", len(paths))
	}
}

func TestFindPaths_GraphNotFound_ReturnsFalse(t *testing.T) {
	store := NewStore()

	_, _, _, ok := store.FindPaths("nonexistent_graph", "n1", "n2", 4, 3)
	if ok {
		t.Error("FindPaths unknown graph: expected false, got true")
	}
}

func TestFindPaths_RespectsMaxDepth(t *testing.T) {
	graphID := "g1"
	store := &Store{
		accounts:     make(map[string]*domain.Account),
		accountUsers: make(map[string][]string),
		userAccount:  make(map[string]string),
		workspaces:   make(map[string]*domain.Workspace),
		documents:    make(map[string]*domain.Document),
		jobs:         make(map[string]*domain.DocumentProcessingJob),
		graphs:       map[string]*domain.Graph{"g1": {GraphID: graphID, WorkspaceID: "ws1"}},
		aliases:      make(map[string]string),
		nodes: map[string][]*domain.Node{
			graphID: {
				{NodeID: "n1", GraphID: graphID},
				{NodeID: "n2", GraphID: graphID},
				{NodeID: "n3", GraphID: graphID},
				{NodeID: "n4", GraphID: graphID},
			},
		},
		edges: map[string][]*domain.Edge{
			graphID: {
				{EdgeID: "e1", GraphID: graphID, SourceNodeID: "n1", TargetNodeID: "n2"},
				{EdgeID: "e2", GraphID: graphID, SourceNodeID: "n2", TargetNodeID: "n3"},
				{EdgeID: "e3", GraphID: graphID, SourceNodeID: "n3", TargetNodeID: "n4"},
			},
		},
	}

	_, _, paths, ok := store.FindPaths(graphID, "n1", "n4", 2, 3)
	if !ok {
		t.Fatal("FindPaths returned false")
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths within maxDepth=2, got %d", len(paths))
	}
}
