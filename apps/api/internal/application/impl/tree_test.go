package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiauth "github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

func TestTreeService_GetTree_ReturnsItems(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	items, err := svc.GetTree(ctx, fixture.Workspace.WorkspaceID, "owner")
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestTreeService_GetTree_EmptyWorkspaceID_ReturnsError(t *testing.T) {
	store := mock.NewStore()
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	_, err := svc.GetTree(context.Background(), "", "owner")
	require.Error(t, err)
}

// 他人が workspace の tree を読もうとしたら Forbidden。
func TestTreeService_GetTree_OtherUser_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	_, err := svc.GetTree(ctx, fixture.Workspace.WorkspaceID, "stranger")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrForbidden), "expected ErrForbidden, got %v", err)
}

// GetSubtree が item を発見し、その workspace が呼び出し側の workspace と一致する場合。
func TestTreeService_GetSubtree_ItemInWorkspace_ReturnsItems(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	items, err := svc.GetSubtree(ctx, fixture.Workspace.WorkspaceID, fixture.RootNodeID, "owner", 3)
	require.NoError(t, err)
	require.NotEmpty(t, items)
	assert.Equal(t, fixture.Workspace.WorkspaceID, items[0].WorkspaceID)
}

// 他人の workspace の item を覗こうとした場合 ErrForbidden を返すことが
// この refactor の核心。NotFound 経由で存在を漏らさない。
func TestTreeService_GetSubtree_ItemInOtherWorkspace_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	// "ws-owner" / "ws-stranger" を作って owner がアクセスできるよう wsOwners に登録する。
	// CreateWorkspace は workspace_root tree_item も同時に作るので、
	// 追加の GetTree 呼び出しは不要。
	acct, _ := store.GetOrCreateAccount(ctx, "owner")
	ownerWS, _ := store.CreateWorkspace(ctx, acct.AccountID, "owner") // → ws-owner
	_, _ = store.CreateWorkspace(ctx, acct.AccountID, "stranger")     // → ws-stranger (owner が membership 持つ)
	ownerNode, _ := store.CreateItem(ctx, ownerWS.WorkspaceID, "n", "", "", "worker")
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	// ownerNode は ws-owner 配下だが、ws-stranger の workspaceID で問い合わせ → Forbidden
	_, err := svc.GetSubtree(ctx, "ws-stranger", ownerNode.ItemID, "owner", 3)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrForbidden), "expected ErrForbidden, got %v", err)
}

func TestTreeService_GetSubtree_MissingArgs_ReturnsError(t *testing.T) {
	store := mock.NewStore()
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	_, err := svc.GetSubtree(context.Background(), "", "i", "owner", 3)
	require.Error(t, err)

	_, err = svc.GetSubtree(context.Background(), "ws", "", "owner", 3)
	require.Error(t, err)
}

// FindPaths は workspace 作成時に root が必ず作られている前提なので、
// 追加の GetOrCreateTree 副作用は不要。
func TestTreeService_FindPaths_FindsTreeFromWorkspace(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	items, _, err := svc.FindPaths(ctx, fixture.Workspace.WorkspaceID, "a", "b", "owner", 3, 5)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestTreeService_FindPaths_MissingArgs_ReturnsError(t *testing.T) {
	store := mock.NewStore()
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	_, _, err := svc.FindPaths(context.Background(), "", "a", "b", "owner", 3, 5)
	require.Error(t, err)
	_, _, err = svc.FindPaths(context.Background(), "ws", "", "b", "owner", 3, 5)
	require.Error(t, err)
	_, _, err = svc.FindPaths(context.Background(), "ws", "a", "", "owner", 3, 5)
	require.Error(t, err)
}

// 公開リンク token (context 経由) で無認証でも tree を read できる。
func TestTreeService_GetTree_ValidShareToken_AllowsRead(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	wsID := fixture.Workspace.WorkspaceID
	require.NoError(t, store.CreateShareLink(ctx, &domain.ShareLink{
		Token: "tok-1", WorkspaceID: wsID, Role: domain.WorkspaceRoleViewer, CreatedBy: "owner",
	}))
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	// userID 空 + share token を context に載せる (無認証ビューア経路)。
	tokenCtx := apiauth.ContextWithShareToken(ctx, "tok-1")
	items, err := svc.GetTree(tokenCtx, wsID, "")
	require.NoError(t, err, "valid share token grants read")
	assert.NotEmpty(t, items)
}

// token が別 workspace を指していたら Forbidden (workspace 不一致)。
func TestTreeService_GetTree_ShareTokenWrongWorkspace_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	require.NoError(t, store.CreateShareLink(ctx, &domain.ShareLink{
		Token: "tok-other", WorkspaceID: "ws-other", Role: domain.WorkspaceRoleViewer, CreatedBy: "owner",
	}))
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	tokenCtx := apiauth.ContextWithShareToken(ctx, "tok-other")
	_, err := svc.GetTree(tokenCtx, fixture.Workspace.WorkspaceID, "")
	assert.ErrorIs(t, err, domain.ErrForbidden, "token for another workspace is rejected")
}

// 無認証 (userID 空) かつ token も無ければ Forbidden。
func TestTreeService_GetTree_NoUserNoToken_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	_, err := svc.GetTree(ctx, fixture.Workspace.WorkspaceID, "")
	assert.ErrorIs(t, err, domain.ErrForbidden, "no auth and no token is rejected")
}

// 失効した token は read できない。
func TestTreeService_GetTree_RevokedShareToken_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	wsID := fixture.Workspace.WorkspaceID
	require.NoError(t, store.CreateShareLink(ctx, &domain.ShareLink{
		Token: "tok-rev", WorkspaceID: wsID, Role: domain.WorkspaceRoleViewer, CreatedBy: "owner",
	}))
	require.NoError(t, store.RevokeShareLink(ctx, wsID, "tok-rev", time.Now().UTC()))
	svc := NewTreeService(TreeServiceDeps{Tree: store, Workspaces: store, Logger: nil})

	tokenCtx := apiauth.ContextWithShareToken(ctx, "tok-rev")
	_, err := svc.GetTree(tokenCtx, wsID, "")
	assert.ErrorIs(t, err, domain.ErrForbidden, "revoked token is rejected")
}
