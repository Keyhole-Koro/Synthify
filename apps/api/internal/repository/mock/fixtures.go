package mock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

type WorkspaceFixture struct {
	UserID    string
	AccountID string
	Workspace *domain.Workspace
	Tree      *domain.Tree
	// RootNodeID is a node seeded directly under the workspace (parent_id
	// IS NULL) by CreateWorkspaceWithTreeFixture. There is no workspace_root
	// item anymore; this is just a convenient handle to an existing node.
	RootNodeID string
	Document   *domain.Document
	Job        *domain.DocumentProcessingJob
}

func CreateUserWorkspaceFixture(t testing.TB, ctx context.Context, store *Store, userID string) WorkspaceFixture {
	t.Helper()

	acct, err := store.GetOrCreateAccount(ctx, userID)
	require.NoError(t, err, "GetOrCreateAccount")

	ws, err := store.CreateWorkspace(ctx, acct.AccountID, "test-workspace")
	require.NoError(t, err, "CreateWorkspace failed")
	require.NotNil(t, ws, "CreateWorkspace returned nil")

	return WorkspaceFixture{
		UserID:    userID,
		AccountID: acct.AccountID,
		Workspace: ws,
	}
}

// CreateWorkspaceWithTreeFixture returns a workspace that already has one
// node seeded directly under it (parent_id IS NULL). The workspace itself is
// the tree root; this seeds a real node so tree-reading tests have content.
func CreateWorkspaceWithTreeFixture(t testing.TB, ctx context.Context, store *Store, userID string) WorkspaceFixture {
	t.Helper()

	fixture := CreateUserWorkspaceFixture(t, ctx, store, userID)
	tree, err := store.GetTree(ctx, fixture.Workspace.WorkspaceID)
	require.NoError(t, err, "GetTree")
	fixture.Tree = tree

	rootNode, err := store.CreateItem(ctx, fixture.Workspace.WorkspaceID, "Root node", "", "", "worker")
	require.NoError(t, err, "CreateItem (root node)")
	require.NotNil(t, rootNode, "CreateItem returned nil")
	fixture.RootNodeID = rootNode.ItemID
	return fixture
}

func CreateWorkspaceWithDocumentFixture(t testing.TB, ctx context.Context, store *Store, userID string) WorkspaceFixture {
	t.Helper()

	fixture := CreateUserWorkspaceFixture(t, ctx, store, userID)
	doc, _, _ := store.CreateDocument(ctx, fixture.Workspace.WorkspaceID, userID, "f.pdf", "application/pdf", 100)
	require.NotNil(t, doc, "CreateDocument returned nil")
	fixture.Document = doc
	return fixture
}

func CreateWorkspaceWithProcessingJobFixture(t testing.TB, ctx context.Context, store *Store, userID string, jobType appv1.JobType) WorkspaceFixture {
	t.Helper()

	fixture := CreateWorkspaceWithTreeFixture(t, ctx, store, userID)
	doc, _, _ := store.CreateDocument(ctx, fixture.Workspace.WorkspaceID, userID, "f.pdf", "application/pdf", 100)
	require.NotNil(t, doc, "CreateDocument returned nil")
	job, err := store.CreateProcessingJob(ctx, doc.DocumentID, fixture.Tree.TreeID, userID, jobType)
	require.NoError(t, err, "CreateProcessingJob failed")
	require.NotNil(t, job, "CreateProcessingJob returned nil")
	fixture.Document = doc
	fixture.Job = job
	return fixture
}
