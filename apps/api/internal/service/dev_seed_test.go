package service

import (
	"context"
	"testing"

	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

func TestDevSeedService_SeedWorkspaceIsIdempotentForUser(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	svc := NewDevSeedService(DevSeedServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Tree:       store,
		Items:      store,
	})

	first, err := svc.SeedWorkspace(ctx, "firebase-user-1")
	if err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if first.Workspace == nil {
		t.Fatal("expected workspace")
	}
	if !first.CreatedWorkspace {
		t.Fatal("expected first seed to create workspace")
	}
	if first.CreatedItemCount != len(devSeedNodes) {
		t.Fatalf("created item count = %d, want %d", first.CreatedItemCount, len(devSeedNodes))
	}
	if first.TotalItemCount != len(devSeedNodes) {
		t.Fatalf("total item count = %d, want %d", first.TotalItemCount, len(devSeedNodes))
	}

	second, err := svc.SeedWorkspace(ctx, "firebase-user-1")
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if second.CreatedWorkspace {
		t.Fatal("expected second seed to reuse workspace")
	}
	if second.CreatedItemCount != 0 {
		t.Fatalf("second created item count = %d, want 0", second.CreatedItemCount)
	}
	if second.TotalItemCount != len(devSeedNodes) {
		t.Fatalf("second total item count = %d, want %d", second.TotalItemCount, len(devSeedNodes))
	}
	if second.Workspace.WorkspaceID != first.Workspace.WorkspaceID {
		t.Fatalf("workspace id = %q, want %q", second.Workspace.WorkspaceID, first.Workspace.WorkspaceID)
	}
}
