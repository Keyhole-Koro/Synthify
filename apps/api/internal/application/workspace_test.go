package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

func TestGetWorkspace_NonMember_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID
	svc := NewWorkspaceService(WorkspaceServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Logger:     nil,
	})

	_, err := svc.GetWorkspace(ctx, wsID, "stranger")
	assert.ErrorIs(t, err, domain.ErrForbidden, "GetWorkspace non-member")
}

func TestGetWorkspace_Member_ReturnsWorkspace(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID
	svc := NewWorkspaceService(WorkspaceServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Logger:     nil,
	})

	got, err := svc.GetWorkspace(ctx, wsID, "owner")
	require.NoError(t, err, "GetWorkspace")
	assert.Equal(t, wsID, got.WorkspaceID, "workspace ID")
}

// 存在しない workspace への access も Forbidden を返す (存在の有無を漏らさない)。
func TestGetWorkspace_UnknownID_ReturnsErrForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	svc := NewWorkspaceService(WorkspaceServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Logger:     nil,
	})

	_, err := svc.GetWorkspace(ctx, "nonexistent_ws", "anyone")
	assert.ErrorIs(t, err, domain.ErrForbidden, "GetWorkspace unknown ID")
}

func TestCreateWorkspace_NoAccount_ReturnsError(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	svc := NewWorkspaceService(WorkspaceServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Logger:     nil,
	})

	_, err := svc.CreateWorkspace(ctx, "my-workspace", "new_user")
	assert.ErrorIs(t, err, domain.ErrNotFound, "CreateWorkspace no account")
}

func TestCreateWorkspace_Success_CreatesWorkspace(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	_, _ = store.CreateAccount(ctx, "owner")
	svc := NewWorkspaceService(WorkspaceServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Logger:     nil,
	})

	ws, err := svc.CreateWorkspace(ctx, "my-workspace", "owner")
	require.NoError(t, err, "CreateWorkspace")
	require.NotNil(t, ws, "CreateWorkspace returned nil workspace")
	assert.Equal(t, "my-workspace", ws.Name, "workspace.Name")
}

func TestWorkspaceMutations_EnforceRoleBoundary(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	wsID := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner").Workspace.WorkspaceID
	svc := NewWorkspaceService(WorkspaceServiceDeps{Accounts: store, Workspaces: store})
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, "editor", domain.WorkspaceRoleEditor, "owner"))
	require.NoError(t, store.UpsertWorkspaceMember(ctx, wsID, "viewer", domain.WorkspaceRoleViewer, "owner"))

	_, err := svc.UpdateWorkspace(ctx, wsID, "editor rename", "editor")
	require.NoError(t, err)
	_, err = svc.UpdateWorkspace(ctx, wsID, "viewer rename", "viewer")
	assert.ErrorIs(t, err, domain.ErrForbidden)
	assert.ErrorIs(t, svc.DeleteWorkspace(ctx, wsID, "editor"), domain.ErrForbidden)
	assert.ErrorIs(t, svc.DeleteWorkspace(ctx, wsID, "viewer"), domain.ErrForbidden)
	require.NoError(t, svc.DeleteWorkspace(ctx, wsID, "owner"))
}
