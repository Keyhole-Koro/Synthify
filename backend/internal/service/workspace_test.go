package service

import (
	"errors"
	"testing"

	"github.com/synthify/backend/internal/domain"
	"github.com/synthify/backend/internal/repository/mock"
)

func TestGetWorkspace_NonMember_ReturnsErrNotFound(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("test ws", "owner_id", "owner@example.com")
	svc := NewWorkspaceService(store)

	_, _, err := svc.GetWorkspace(ws.WorkspaceID, "stranger")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetWorkspace non-member: err = %v, want ErrNotFound", err)
	}
}

func TestGetWorkspace_Member_ReturnsWorkspace(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("test ws", "owner_id", "owner@example.com")
	svc := NewWorkspaceService(store)

	got, members, err := svc.GetWorkspace(ws.WorkspaceID, "owner_id")
	if err != nil {
		t.Fatalf("GetWorkspace: unexpected error: %v", err)
	}
	if got.WorkspaceID != ws.WorkspaceID {
		t.Errorf("workspace ID = %q, want %q", got.WorkspaceID, ws.WorkspaceID)
	}
	if len(members) == 0 {
		t.Error("expected at least one member")
	}
}

func TestGetWorkspace_UnknownID_ReturnsErrNotFound(t *testing.T) {
	store := mock.NewStore()
	svc := NewWorkspaceService(store)

	_, _, err := svc.GetWorkspace("nonexistent_ws", "anyone")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetWorkspace unknown ID: err = %v, want ErrNotFound", err)
	}
}

func TestTransferOwnership_UpdatesOwnerAndRoles(t *testing.T) {
	store := mock.NewStore()
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

	svc := NewWorkspaceService(store)
	updatedWS, updatedMembers, err := svc.TransferOwnership(ws.WorkspaceID, userBID)
	if err != nil {
		t.Fatalf("TransferOwnership: unexpected error: %v", err)
	}
	if updatedWS.OwnerID != userBID {
		t.Errorf("workspace.OwnerID = %q, want %q", updatedWS.OwnerID, userBID)
	}

	roleOf := func(userID string) domain.WorkspaceRole {
		for _, m := range updatedMembers {
			if m.UserID == userID {
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

func TestTransferOwnership_NonMemberNewOwner_ReturnsErrNotFound(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "user_a", "a@example.com")
	svc := NewWorkspaceService(store)

	_, _, err := svc.TransferOwnership(ws.WorkspaceID, "nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("TransferOwnership non-member: err = %v, want ErrNotFound", err)
	}
}

func TestInviteMember_UnknownWorkspace_ReturnsErrNotFound(t *testing.T) {
	store := mock.NewStore()
	svc := NewWorkspaceService(store)

	_, err := svc.InviteMember("nonexistent_ws", "x@example.com", "editor", false)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("InviteMember unknown ws: err = %v, want ErrNotFound", err)
	}
}

func TestRemoveMember_UnknownMember_ReturnsErrNotFound(t *testing.T) {
	store := mock.NewStore()
	ws := store.CreateWorkspace("ws", "owner", "o@example.com")
	svc := NewWorkspaceService(store)

	err := svc.RemoveMember(ws.WorkspaceID, "nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("RemoveMember unknown member: err = %v, want ErrNotFound", err)
	}
}

func TestListWorkspaces_ReturnsOnlyUserWorkspaces(t *testing.T) {
	store := mock.NewStore()
	store.CreateWorkspace("ws_a", "user_a", "a@example.com")
	store.CreateWorkspace("ws_b", "user_b", "b@example.com")
	svc := NewWorkspaceService(store)

	got := svc.ListWorkspaces("user_a")
	if len(got) != 1 {
		t.Errorf("ListWorkspaces user_a: got %d workspaces, want 1", len(got))
	}
	if got[0].OwnerID != "user_a" {
		t.Errorf("workspace owner = %q, want user_a", got[0].OwnerID)
	}
}
