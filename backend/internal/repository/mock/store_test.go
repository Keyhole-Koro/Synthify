package mock

import (
	"testing"

	"github.com/synthify/backend/internal/domain"
)

// ── TransferOwnership ─────────────────────────────────────────────────────────

func TestTransferOwnership_SwapsRoles(t *testing.T) {
	store := NewStore()
	ws := store.CreateWorkspace("ws", "user_a", "a@example.com")
	store.InviteMember(ws.WorkspaceID, "b@example.com", string(domain.WorkspaceRoleEditor), false)

	_, members, _ := store.GetWorkspace(ws.WorkspaceID)
	var userBID string
	for _, m := range members {
		if m.Email == "b@example.com" {
			userBID = m.UserID
		}
	}
	if userBID == "" {
		t.Fatal("could not find invited member user B")
	}

	updatedWS, updatedMembers, ok := store.TransferOwnership(ws.WorkspaceID, userBID)
	if !ok {
		t.Fatal("TransferOwnership returned false")
	}
	if updatedWS.OwnerID != userBID {
		t.Errorf("OwnerID = %q, want %q", updatedWS.OwnerID, userBID)
	}

	roleOf := func(uid string) domain.WorkspaceRole {
		for _, m := range updatedMembers {
			if m.UserID == uid {
				return m.Role
			}
		}
		return ""
	}
	if roleOf(userBID) != domain.WorkspaceRoleOwner {
		t.Errorf("new owner role = %q, want owner", roleOf(userBID))
	}
	if roleOf("user_a") != domain.WorkspaceRoleEditor {
		t.Errorf("old owner role = %q, want editor", roleOf("user_a"))
	}
}

func TestTransferOwnership_NonMemberNewOwner_ReturnsFalse(t *testing.T) {
	store := NewStore()
	ws := store.CreateWorkspace("ws", "user_a", "a@example.com")

	_, _, ok := store.TransferOwnership(ws.WorkspaceID, "nobody")
	if ok {
		t.Error("TransferOwnership with non-member: expected false, got true")
	}
}

func TestTransferOwnership_UnknownWorkspace_ReturnsFalse(t *testing.T) {
	store := NewStore()

	_, _, ok := store.TransferOwnership("nonexistent_ws", "anyone")
	if ok {
		t.Error("TransferOwnership unknown workspace: expected false, got true")
	}
}

// ── RemoveMember ──────────────────────────────────────────────────────────────

func TestRemoveMember_RemovesMemberFromList(t *testing.T) {
	store := NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")
	store.InviteMember(ws.WorkspaceID, "member@example.com", "editor", false)

	_, members, _ := store.GetWorkspace(ws.WorkspaceID)
	var memberID string
	for _, m := range members {
		if m.Email == "member@example.com" {
			memberID = m.UserID
		}
	}

	ok := store.RemoveMember(ws.WorkspaceID, memberID)
	if !ok {
		t.Fatal("RemoveMember returned false")
	}
	if store.IsWorkspaceMember(ws.WorkspaceID, memberID) {
		t.Error("member still present after removal")
	}
}

func TestRemoveMember_UnknownMember_ReturnsFalse(t *testing.T) {
	store := NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")

	ok := store.RemoveMember(ws.WorkspaceID, "nobody")
	if ok {
		t.Error("RemoveMember unknown member: expected false, got true")
	}
}

// ── ApproveAlias / RejectAlias ────────────────────────────────────────────────

func TestApproveAlias_RecordsAlias(t *testing.T) {
	store := NewStore()
	ws := store.CreateWorkspace("ws", "u1", "u@example.com")
	doc, _ := store.CreateDocument(ws.WorkspaceID, "f.pdf", "application/pdf", 100)
	store.StartProcessing(doc.DocumentID, false, "full")

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
	ws := store.CreateWorkspace("ws", "u1", "u@example.com")

	ok := store.ApproveAlias(ws.WorkspaceID, "nonexistent", "also_nonexistent")
	if ok {
		t.Error("ApproveAlias unknown nodes: expected false, got true")
	}
}

func TestRejectAlias_RemovesAlias(t *testing.T) {
	store := NewStore()
	ws := store.CreateWorkspace("ws", "u1", "u@example.com")
	doc, _ := store.CreateDocument(ws.WorkspaceID, "f.pdf", "application/pdf", 100)
	store.StartProcessing(doc.DocumentID, false, "full")

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
	store := &Store{
		workspaces: make(map[string]*domain.Workspace),
		members:    make(map[string][]*domain.WorkspaceMember),
		documents:  make(map[string]*domain.Document),
		aliases:    make(map[string]string),
		views:      make(map[string][]viewRecord),
		nodes: map[string][]*domain.Node{
			"doc1": {
				{NodeID: "n1", DocumentID: "doc1"},
				{NodeID: "n2", DocumentID: "doc1"},
				{NodeID: "n3", DocumentID: "doc1"},
			},
		},
		edges: map[string][]*domain.Edge{
			"doc1": {
				{EdgeID: "e1", DocumentID: "doc1", SourceNodeID: "n1", TargetNodeID: "n2"},
				{EdgeID: "e2", DocumentID: "doc1", SourceNodeID: "n2", TargetNodeID: "n3"},
			},
		},
	}

	_, _, paths, ok := store.FindPaths("doc1", "n1", "n3", 4, 3)
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
	// n3 is not connected to n1/n2.
	store := &Store{
		workspaces: make(map[string]*domain.Workspace),
		members:    make(map[string][]*domain.WorkspaceMember),
		documents:  make(map[string]*domain.Document),
		aliases:    make(map[string]string),
		views:      make(map[string][]viewRecord),
		nodes: map[string][]*domain.Node{
			"doc1": {
				{NodeID: "n1", DocumentID: "doc1"},
				{NodeID: "n2", DocumentID: "doc1"},
				{NodeID: "n3", DocumentID: "doc1"},
			},
		},
		edges: map[string][]*domain.Edge{
			"doc1": {
				{EdgeID: "e1", DocumentID: "doc1", SourceNodeID: "n1", TargetNodeID: "n2"},
			},
		},
	}

	_, _, paths, ok := store.FindPaths("doc1", "n1", "n3", 4, 3)
	if !ok {
		t.Fatal("FindPaths returned false (document exists)")
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths, got %d", len(paths))
	}
}

func TestFindPaths_DocumentNotFound_ReturnsFalse(t *testing.T) {
	store := NewStore()

	_, _, _, ok := store.FindPaths("nonexistent", "n1", "n2", 4, 3)
	if ok {
		t.Error("FindPaths unknown document: expected false, got true")
	}
}

func TestFindPaths_RespectsMaxDepth(t *testing.T) {
	// n1 → n2 → n3 → n4: path length 3, should be blocked at maxDepth=2.
	store := &Store{
		workspaces: make(map[string]*domain.Workspace),
		members:    make(map[string][]*domain.WorkspaceMember),
		documents:  make(map[string]*domain.Document),
		aliases:    make(map[string]string),
		views:      make(map[string][]viewRecord),
		nodes: map[string][]*domain.Node{
			"doc1": {
				{NodeID: "n1", DocumentID: "doc1"},
				{NodeID: "n2", DocumentID: "doc1"},
				{NodeID: "n3", DocumentID: "doc1"},
				{NodeID: "n4", DocumentID: "doc1"},
			},
		},
		edges: map[string][]*domain.Edge{
			"doc1": {
				{EdgeID: "e1", DocumentID: "doc1", SourceNodeID: "n1", TargetNodeID: "n2"},
				{EdgeID: "e2", DocumentID: "doc1", SourceNodeID: "n2", TargetNodeID: "n3"},
				{EdgeID: "e3", DocumentID: "doc1", SourceNodeID: "n3", TargetNodeID: "n4"},
			},
		},
	}

	_, _, paths, ok := store.FindPaths("doc1", "n1", "n4", 2, 3)
	if !ok {
		t.Fatal("FindPaths returned false")
	}
	if len(paths) != 0 {
		t.Errorf("expected no paths within maxDepth=2, got %d", len(paths))
	}
}
