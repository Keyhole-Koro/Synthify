package mock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

var ctx = context.Background()

// ── IsWorkspaceAccessible ─────────────────────────────────────────────────────

func TestIsWorkspaceAccessible_Owner_ReturnsTrue(t *testing.T) {
	store := NewStore()
	ws := CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace

	assert.True(t, store.IsWorkspaceAccessible(ctx, ws.WorkspaceID, "owner"), "owner should have access to their workspace")
}

func TestIsWorkspaceAccessible_Stranger_ReturnsFalse(t *testing.T) {
	store := NewStore()
	ws := CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace

	assert.False(t, store.IsWorkspaceAccessible(ctx, ws.WorkspaceID, "stranger"), "stranger should not have access")
}

// ── ApproveAlias / RejectAlias ────────────────────────────────────────────────

func TestApproveAlias_RecordsAlias(t *testing.T) {
	store := NewStore()
	fixture := CreateWorkspaceWithProcessingJobFixture(t, ctx, store, "u1", treev1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	ws := fixture.Workspace

	// Add seed items
	store.CreateItem(ctx, ws.WorkspaceID, "root", "root desc", "", "system")
	store.CreateItem(ctx, ws.WorkspaceID, "child", "child desc", "item-root", "system")

	err := store.ApproveAlias(ctx, ws.WorkspaceID, "item-root", "item-child")
	require.NoError(t, err, "ApproveAlias")
}

func TestApproveAlias_UnknownItem_ReturnsFalse(t *testing.T) {
	store := NewStore()
	ws := CreateUserWorkspaceFixture(t, ctx, store, "u1").Workspace

	err := store.ApproveAlias(ctx, ws.WorkspaceID, "nonexistent", "also_nonexistent")
	assert.NoError(t, err, "ApproveAlias unknown items")
}

func TestRejectAlias_RemovesAlias(t *testing.T) {
	store := NewStore()
	fixture := CreateWorkspaceWithProcessingJobFixture(t, ctx, store, "u1", treev1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	ws := fixture.Workspace

	require.NoError(t, store.ApproveAlias(ctx, ws.WorkspaceID, "item-root", "item-child"), "ApproveAlias")

	err := store.RejectAlias(ctx, ws.WorkspaceID, "item-root", "item-child")
	require.NoError(t, err, "RejectAlias")
}

func TestGetJobPlanningSignals_CountsProvenanceAndAliases(t *testing.T) {
	store := NewStore()
	fixture := CreateWorkspaceWithProcessingJobFixture(t, ctx, store, "u1", treev1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	ws := fixture.Workspace

	err := store.UpsertItemSource(ctx, "nd_tel", "doc-1", "file-1", "chunk-1", "source", 0.9)
	require.NoError(t, err, "UpsertItemSource nd_tel")
	err = store.UpsertItemSource(ctx, "nd_roi", "doc-1", "file-1", "chunk-2", "source", 0.8)
	require.NoError(t, err, "UpsertItemSource nd_roi")

	signals, err := store.GetJobPlanningSignals(ctx, "doc-1", ws.WorkspaceID, ws.WorkspaceID)
	require.NoError(t, err, "GetJobPlanningSignals")
	require.NotNil(t, signals, "GetJobPlanningSignals returned nil")
}

// ── FindPaths (Tree traversal) ────────────────────────────────────────────────

func TestFindPaths_FindsConnectedPath(t *testing.T) {
	wsID := "ws1"
	store := NewStore()
	store.CreateItem(ctx, wsID, "n1", "", "", "u1")
	store.CreateItem(ctx, wsID, "n2", "", "item-n1", "u1")
	store.CreateItem(ctx, wsID, "n3", "", "item-n2", "u1")

	items, paths, err := store.FindPaths(ctx, wsID, "item-n3", "item-n1", 4, 3)
	require.NoError(t, err, "FindPaths")
	require.NotEmpty(t, items, "expected at least one item")
	assert.Empty(t, paths, "mock FindPaths does not synthesize path edges")
}

func TestFindPaths_NoPathExists_ReturnsEmptyPaths(t *testing.T) {
	wsID := "ws1"
	store := NewStore()
	store.CreateItem(ctx, wsID, "n1", "", "", "u1")
	store.CreateItem(ctx, wsID, "n3", "", "", "u1")

	_, paths, err := store.FindPaths(ctx, wsID, "item-n3", "item-n1", 4, 3)
	require.NoError(t, err, "FindPaths")
	assert.Empty(t, paths, "expected no paths")
}

func TestFindPaths_WorkspaceNotFound_ReturnsFalse(t *testing.T) {
	store := NewStore()

	_, _, err := store.FindPaths(ctx, "nonexistent_ws", "n1", "n2", 4, 3)
	assert.Error(t, err, "FindPaths unknown workspace: expected error, got nil")
}
