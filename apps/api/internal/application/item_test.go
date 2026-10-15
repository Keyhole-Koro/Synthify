package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	"github.com/synthify/backend/internal/domain"
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

func TestGetItemEvidence_ReturnsSourcesAndSourceRefs(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := NewItemService(ItemServiceDeps{
		Repo:       repo,
		Workspaces: repo,
		Logger:     nil,
	})
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID
	item, err := svc.CreateItem(ctx, wsID, "api-design", "", "", "owner")
	require.NoError(t, err)

	require.NoError(t, repo.UpsertItemSource(ctx, item.ItemID, "doc-1", "file-1", "chunk-1", "source text", 0.8))
	require.NoError(t, repo.UpsertItemSourceRef(ctx, &domain.ItemSourceRef{
		ItemID:       item.ItemID,
		Type:         "github",
		URL:          "https://github.com/owner/repo/blob/abc123/docs/spec.md#L10-L42",
		Repo:         "owner/repo",
		Ref:          "abc123",
		Path:         "docs/spec.md",
		LineStart:    10,
		LineEnd:      42,
		Kind:         "file",
		Confidence:   0.92,
		MetadataJSON: "{}",
	}))

	evidence, err := svc.GetItemEvidence(ctx, item.ItemID, wsID, "owner")
	require.NoError(t, err)
	require.Len(t, evidence.Sources, 1)
	assert.Equal(t, "doc-1", evidence.Sources[0].DocumentID)
	require.Len(t, evidence.SourceRefs, 1)
	assert.Equal(t, "github", evidence.SourceRefs[0].Type)
	assert.Equal(t, "owner/repo", evidence.SourceRefs[0].Repo)
	assert.Equal(t, "docs/spec.md", evidence.SourceRefs[0].Path)
	assert.Equal(t, 10, evidence.SourceRefs[0].LineStart)
	assert.Equal(t, 42, evidence.SourceRefs[0].LineEnd)
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

func newAliasFixture(t *testing.T) (context.Context, *mock.Store, *ItemService, string, *domain.Item, *domain.Item) {
	t.Helper()
	ctx := context.Background()
	repo := mock.NewStore()
	svc := NewItemService(ItemServiceDeps{Repo: repo, Workspaces: repo})
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID
	canonical, err := repo.CreateItem(ctx, wsID, "canonical", "", "", "owner")
	require.NoError(t, err)
	alias, err := repo.CreateItem(ctx, wsID, "alias", "", "", "owner")
	require.NoError(t, err)
	return ctx, repo, svc, wsID, canonical, alias
}

func TestApproveAlias_ItemsInWorkspace_Succeeds(t *testing.T) {
	ctx, _, svc, wsID, canonical, alias := newAliasFixture(t)

	require.NoError(t, svc.ApproveAlias(ctx, wsID, canonical.ItemID, alias.ItemID, "owner"))
	require.NoError(t, svc.RejectAlias(ctx, wsID, canonical.ItemID, alias.ItemID, "owner"))
}

// Write access to one's own workspace must not allow recording an alias that
// points at another workspace's item.
func TestApproveAlias_ItemFromOtherWorkspace_ReturnsForbidden(t *testing.T) {
	ctx, repo, svc, wsID, canonical, alias := newAliasFixture(t)
	// The mock derives workspace IDs from the name, so the other workspace needs
	// its own name; CreateUserWorkspaceFixture would overwrite the first one.
	otherAccount, err := repo.GetOrCreateAccount(ctx, "someone-else")
	require.NoError(t, err)
	otherWorkspace, err := repo.CreateWorkspace(ctx, otherAccount.AccountID, "other-workspace")
	require.NoError(t, err)
	require.NotEqual(t, wsID, otherWorkspace.WorkspaceID)
	foreign, err := repo.CreateItem(ctx, otherWorkspace.WorkspaceID, "foreign", "", "", "someone-else")
	require.NoError(t, err)
	// The caller can still write to their own workspace; only the pair is wrong.
	require.NoError(t, svc.ApproveAlias(ctx, wsID, canonical.ItemID, alias.ItemID, "owner"))

	assert.ErrorIs(t, svc.ApproveAlias(ctx, wsID, canonical.ItemID, foreign.ItemID, "owner"), domain.ErrForbidden)
	assert.ErrorIs(t, svc.RejectAlias(ctx, wsID, foreign.ItemID, canonical.ItemID, "owner"), domain.ErrForbidden)
}

func TestApproveAlias_UnknownItem_ReturnsNotFound(t *testing.T) {
	ctx, _, svc, wsID, canonical, _ := newAliasFixture(t)

	assert.ErrorIs(t, svc.ApproveAlias(ctx, wsID, canonical.ItemID, "missing-item", "owner"), domain.ErrNotFound)
}

func TestApproveAlias_SameItem_ReturnsInvalidArgument(t *testing.T) {
	ctx, _, svc, wsID, canonical, _ := newAliasFixture(t)

	assert.ErrorIs(t, svc.ApproveAlias(ctx, wsID, canonical.ItemID, canonical.ItemID, "owner"), domain.ErrInvalidArgument)
}

func TestApproveAlias_Viewer_ReturnsForbidden(t *testing.T) {
	ctx, repo, svc, wsID, canonical, alias := newAliasFixture(t)
	require.NoError(t, repo.UpsertWorkspaceMember(ctx, wsID, "viewer", domain.WorkspaceRoleViewer, "owner"))

	assert.ErrorIs(t, svc.ApproveAlias(ctx, wsID, canonical.ItemID, alias.ItemID, "viewer"), domain.ErrForbidden)
}
