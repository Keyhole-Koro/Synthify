package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
)

func TestTreeService_GetTree_ReturnsItems(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	svc := NewTreeService(store, nil)

	items, err := svc.GetTree(ctx, fixture.Workspace.WorkspaceID)
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestTreeService_GetTree_EmptyWorkspaceID_ReturnsError(t *testing.T) {
	store := mock.NewStore()
	svc := NewTreeService(store, nil)

	_, err := svc.GetTree(context.Background(), "")
	require.Error(t, err)
}

// GetSubtree が item を発見し、その workspace が呼び出し側の workspace と一致する場合。
func TestTreeService_GetSubtree_ItemInWorkspace_ReturnsItems(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateWorkspaceWithTreeFixture(t, ctx, store, "owner")
	svc := NewTreeService(store, nil)

	items, err := svc.GetSubtree(ctx, fixture.Workspace.WorkspaceID, "nd_root", 3)
	require.NoError(t, err)
	require.NotEmpty(t, items)
	assert.Equal(t, fixture.Workspace.WorkspaceID, items[0].WorkspaceID)
}

// 他人の workspace の item を覗こうとした場合 ErrForbidden を返すことが
// この refactor の核心。NotFound 経由で存在を漏らさない。
func TestTreeService_GetSubtree_ItemInOtherWorkspace_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	// owner の workspace に root item を作る (GetOrCreateTree 経由)
	_, err := store.GetOrCreateTree(ctx, "ws-owner")
	require.NoError(t, err)
	svc := NewTreeService(store, nil)

	// nd_root は ws-owner 配下だが、別の workspaceID を渡す → Forbidden
	_, err = svc.GetSubtree(ctx, "ws-stranger", "nd_root", 3)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrForbidden), "expected ErrForbidden, got %v", err)
}

func TestTreeService_GetSubtree_MissingArgs_ReturnsError(t *testing.T) {
	store := mock.NewStore()
	svc := NewTreeService(store, nil)

	_, err := svc.GetSubtree(context.Background(), "", "i", 3)
	require.Error(t, err)

	_, err = svc.GetSubtree(context.Background(), "ws", "", 3)
	require.Error(t, err)
}

// FindPaths は tree が無くても GetOrCreateTree の副作用で作成して進む。
func TestTreeService_FindPaths_AutoCreatesTree(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	svc := NewTreeService(store, nil)

	items, _, err := svc.FindPaths(ctx, "ws-fresh", "a", "b", 3, 5)
	require.NoError(t, err)
	// 作成された tree には root だけ存在する
	assert.NotEmpty(t, items)
}

func TestTreeService_FindPaths_MissingArgs_ReturnsError(t *testing.T) {
	store := mock.NewStore()
	svc := NewTreeService(store, nil)

	_, _, err := svc.FindPaths(context.Background(), "", "a", "b", 3, 5)
	require.Error(t, err)
	_, _, err = svc.FindPaths(context.Background(), "ws", "", "b", 3, 5)
	require.Error(t, err)
	_, _, err = svc.FindPaths(context.Background(), "ws", "a", "", 3, 5)
	require.Error(t, err)
}
