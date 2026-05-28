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
