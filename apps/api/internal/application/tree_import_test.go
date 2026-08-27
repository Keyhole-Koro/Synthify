package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

const importDocument = `#STIF/1
@tree mode="append" scope="canonical"

::item /overview
title: "Overview"
description: "Everything at a glance"
order: 10
---
<article><h1>Overview</h1></article>
::css
article { line-height: 1.7; }
::end

::item /overview/api
title: "API Design"
parent: /overview
state: pending_review
source:
  - type: github
    repo: example/synthify
    ref: abc123
    path: docs/api.md
    lines: [12, 48]
    confidence: 0.86
---
<p>API notes.</p>
::end
`

func newImportService(repo *mock.Store) *TreeImportService {
	return NewTreeImportService(TreeImportServiceDeps{
		Items:      repo,
		Workspaces: repo,
		Transactor: repo,
		Logger:     nil,
	})
}

func importedByLocalPath(t *testing.T, result *ImportResult, localPath string) ImportedItem {
	t.Helper()
	for _, item := range result.Items {
		if item.LocalPath == localPath {
			return item
		}
	}
	t.Fatalf("imported item %s not found", localPath)
	return ImportedItem{}
}

func TestImportStif_CommitsItems(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID

	result, err := svc.ImportStif(ctx, wsID, importDocument, "owner", false)
	require.NoError(t, err)
	require.True(t, result.Committed, "diagnostics: %+v", result.Diagnostics)
	assert.Equal(t, 2, result.ImportedCount)
	assert.Equal(t, "append", result.Mode)

	overview := importedByLocalPath(t, result, "/overview")
	require.NotEmpty(t, overview.ItemID, "committed items must carry their new id")

	stored, err := repo.GetItem(ctx, overview.ItemID)
	require.NoError(t, err)
	assert.Equal(t, "Overview", stored.Title)
	assert.Equal(t, "owner", stored.CreatedBy, "created_by is the importing user")
	assert.Contains(t, stored.Content, "<h1>Overview</h1>")
	assert.Contains(t, stored.OverrideCSS, "line-height")
	assert.Empty(t, stored.ParentID, "a root item has no parent")
	assert.Equal(t, appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_HUMAN_CURATED, stored.GovernanceState,
		"an item with no explicit state defaults to human_curated")
}

func TestImportStif_ResolvesParentLinks(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID

	result, err := svc.ImportStif(ctx, wsID, importDocument, "owner", false)
	require.NoError(t, err)
	require.True(t, result.Committed)

	overview := importedByLocalPath(t, result, "/overview")
	api := importedByLocalPath(t, result, "/overview/api")

	stored, err := repo.GetItem(ctx, api.ItemID)
	require.NoError(t, err)
	assert.Equal(t, overview.ItemID, stored.ParentID, "the child points at the parent's assigned id")
	assert.Equal(t, 1, stored.Level, "level follows the item's depth in the imported tree")
	assert.Equal(t, appv1.ItemGovernanceState_ITEM_GOVERNANCE_STATE_PENDING_REVIEW, stored.GovernanceState)
}

func TestImportStif_StoresSourceRefs(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")

	result, err := svc.ImportStif(ctx, fixture.Workspace.WorkspaceID, importDocument, "owner", false)
	require.NoError(t, err)
	require.True(t, result.Committed)

	api := importedByLocalPath(t, result, "/overview/api")
	refs, err := repo.ListItemSourceRefs(ctx, api.ItemID)
	require.NoError(t, err)
	require.Len(t, refs, 1)

	assert.Equal(t, "github", refs[0].Type)
	assert.Equal(t, "example/synthify", refs[0].Repo)
	assert.Equal(t, "abc123", refs[0].Ref)
	assert.Equal(t, "docs/api.md", refs[0].Path)
	assert.Equal(t, 12, refs[0].LineStart)
	assert.Equal(t, 48, refs[0].LineEnd)
	assert.InDelta(t, 0.86, refs[0].Confidence, 0.0001)
}

// A dry run is what the preview UI calls; it must be able to show exactly what
// would be written without writing any of it.
func TestImportStif_DryRunWritesNothing(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID

	result, err := svc.ImportStif(ctx, wsID, importDocument, "owner", true)
	require.NoError(t, err)

	assert.False(t, result.Committed)
	assert.Zero(t, result.ImportedCount)
	assert.Len(t, result.Items, 2, "the preview still describes every item")
	for _, item := range result.Items {
		assert.Empty(t, item.ItemID, "a dry run assigns no ids")
	}

	tree, err := repo.GetTreeByWorkspace(ctx, wsID)
	require.NoError(t, err)
	assert.Empty(t, tree, "a dry run must not touch the tree")
}

// A document that does not parse is an ordinary outcome the UI renders, not an
// RPC failure.
func TestImportStif_InvalidDocumentReportsDiagnosticsWithoutError(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID

	result, err := svc.ImportStif(ctx, wsID, "::item /a\ntitle: A\n::end\n", "owner", false)
	require.NoError(t, err, "a malformed document is not an RPC error")

	assert.False(t, result.Committed)
	assert.NotEmpty(t, result.Diagnostics)

	tree, err := repo.GetTreeByWorkspace(ctx, wsID)
	require.NoError(t, err)
	assert.Empty(t, tree, "nothing is written when the document has errors")
}

// The whole document lands or none of it does: a partially imported tree has
// items whose parents were never written.
func TestImportStif_UnresolvedParentWritesNothing(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID

	src := "#STIF/1\n" +
		"::item /a\ntitle: A\n::end\n" +
		"::item /b\ntitle: B\nparent: /missing\n::end\n"
	result, err := svc.ImportStif(ctx, wsID, src, "owner", false)
	require.NoError(t, err)
	assert.False(t, result.Committed)

	tree, err := repo.GetTreeByWorkspace(ctx, wsID)
	require.NoError(t, err)
	assert.Empty(t, tree, "the valid item must not be imported on its own")
}

func TestImportStif_SanitizesContentBeforeStoring(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")

	src := "#STIF/1\n::item /a\ntitle: A\n---\n<p onclick=\"x()\">text</p><script>steal()</script>\n::end\n"
	result, err := svc.ImportStif(ctx, fixture.Workspace.WorkspaceID, src, "owner", false)
	require.NoError(t, err)
	require.True(t, result.Committed, "sanitizing warns, it does not block")

	stored, err := repo.GetItem(ctx, result.Items[0].ItemID)
	require.NoError(t, err)
	assert.NotContains(t, stored.Content, "script")
	assert.NotContains(t, stored.Content, "onclick")
	assert.Contains(t, stored.Content, "text")
}

func TestImportStif_WarnsOnWorkspaceMismatch(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")

	src := "#STIF/1\n@tree workspace=\"some-other-ws\"\n::item /a\ntitle: A\n::end\n"
	result, err := svc.ImportStif(ctx, fixture.Workspace.WorkspaceID, src, "owner", true)
	require.NoError(t, err)

	var found bool
	for _, d := range result.Diagnostics {
		if d.Code == "workspace_mismatch" {
			found = true
		}
	}
	assert.True(t, found, "diagnostics: %+v", result.Diagnostics)
}

func TestImportStif_Viewer_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID
	require.NoError(t, repo.UpsertWorkspaceMember(ctx, wsID, "viewer", domain.WorkspaceRoleViewer, "owner"))

	_, err := svc.ImportStif(ctx, wsID, importDocument, "viewer", false)
	assert.ErrorIs(t, err, domain.ErrForbidden, "a viewer cannot import")
}

// Read permission is not import permission, and a dry run still reads the
// workspace, so it has to be gated the same way.
func TestImportStif_Viewer_DryRunAlsoForbidden(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")
	wsID := fixture.Workspace.WorkspaceID
	require.NoError(t, repo.UpsertWorkspaceMember(ctx, wsID, "viewer", domain.WorkspaceRoleViewer, "owner"))

	_, err := svc.ImportStif(ctx, wsID, importDocument, "viewer", true)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestImportStif_NonMember_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")

	_, err := svc.ImportStif(ctx, fixture.Workspace.WorkspaceID, importDocument, "stranger", false)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// Sibling order is stored as created_at, so the service has to write siblings
// in the order the document asked for.
func TestImportStif_WritesSiblingsInOrder(t *testing.T) {
	ctx := context.Background()
	repo := mock.NewStore()
	svc := newImportService(repo)
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, repo, "owner")

	src := "#STIF/1\n" +
		"::item /root\ntitle: Root\n::end\n" +
		"::item /root/c\ntitle: C\norder: 30\n::end\n" +
		"::item /root/a\ntitle: A\norder: 10\n::end\n" +
		"::item /root/b\ntitle: B\norder: 20\n::end\n"
	result, err := svc.ImportStif(ctx, fixture.Workspace.WorkspaceID, src, "owner", false)
	require.NoError(t, err)
	require.True(t, result.Committed, "diagnostics: %+v", result.Diagnostics)

	titles := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		titles = append(titles, item.Title)
	}
	assert.Equal(t, []string{"Root", "A", "B", "C"}, titles)
}
