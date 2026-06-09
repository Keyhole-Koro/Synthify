package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

func TestCreateItem_CreatesItem(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := NewItemService(ItemServiceDeps{
		Repo:       repo,
		Workspaces: repo,
		Logger:     nil,
	})

	// fixture: owner ユーザーで workspace を作る (acct id == user id)。
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")

	item, err := svc.CreateItem(ctx, fixture.Workspace.WorkspaceID, "root", "root desc", "", "owner")
	require.NoError(t, err, "CreateItem")
	assert.NotNil(t, item, "expected item, got nil")
}

func TestCreateItem_NonMember_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := NewItemService(ItemServiceDeps{
		Repo:       repo,
		Workspaces: repo,
		Logger:     nil,
	})
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")

	_, err := svc.CreateItem(ctx, fixture.Workspace.WorkspaceID, "root", "", "", "stranger")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestCreateItem_Viewer_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := NewItemService(ItemServiceDeps{
		Repo:       repo,
		Workspaces: repo,
		Logger:     nil,
	})
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID
	require.NoError(t, repo.UpsertWorkspaceMember(ctx, wsID, "viewer", domain.WorkspaceRoleViewer, "owner"))

	_, err := svc.CreateItem(ctx, wsID, "root", "", "", "viewer")
	assert.ErrorIs(t, err, domain.ErrForbidden, "viewer cannot create items")
}

func TestCreateItem_Editor_Succeeds(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := NewItemService(ItemServiceDeps{
		Repo:       repo,
		Workspaces: repo,
		Logger:     nil,
	})
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID
	require.NoError(t, repo.UpsertWorkspaceMember(ctx, wsID, "editor", domain.WorkspaceRoleEditor, "owner"))

	item, err := svc.CreateItem(ctx, wsID, "root", "desc", "", "editor")
	require.NoError(t, err, "editor can create items")
	assert.NotNil(t, item)
}

// viewer は書き込みは不可だが read はできる (共有の閲覧専用が成立する)。
func TestGetItem_Viewer_HasReadAccess(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := NewItemService(ItemServiceDeps{
		Repo:       repo,
		Workspaces: repo,
		Logger:     nil,
	})
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID
	created, err := svc.CreateItem(ctx, wsID, "root", "", "", "owner")
	require.NoError(t, err, "owner CreateItem")
	require.NoError(t, repo.UpsertWorkspaceMember(ctx, wsID, "viewer", domain.WorkspaceRoleViewer, "owner"))

	got, err := svc.GetItem(ctx, created.ItemID, wsID, "viewer")
	require.NoError(t, err, "viewer can read items")
	assert.Equal(t, created.ItemID, got.ItemID)
}

func TestGetItem_OtherWorkspaceID_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := NewItemService(ItemServiceDeps{
		Repo:       repo,
		Workspaces: repo,
		Logger:     nil,
	})
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	// item を作って別 workspaceID を query で渡す
	item, err := repo.CreateItem(ctx, fixture.Workspace.WorkspaceID, "x", "", "", "owner")
	require.NoError(t, err)
	require.NotNil(t, item)

	_, err = svc.GetItem(ctx, item.ItemID, "ws-bogus", "owner")
	require.Error(t, err)
	// 'owner' は ws-bogus の member ではないので Forbidden が先に出る
	assert.ErrorIs(t, err, domain.ErrForbidden)
}
