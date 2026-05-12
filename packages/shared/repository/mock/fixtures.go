package mock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/packages/shared/domain"
	treev1 "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1"
)

type WorkspaceFixture struct {
	UserID    string
	AccountID string
	Workspace *domain.Workspace
	Tree      *domain.Tree
	Document  *domain.Document
	Job       *domain.DocumentProcessingJob
}

func CreateUserWorkspaceFixture(t testing.TB, ctx context.Context, store *Store, userID string) WorkspaceFixture {
	t.Helper()

	acct, err := store.GetOrCreateAccount(ctx, userID)
	require.NoError(t, err, "GetOrCreateAccount")

	ws := store.CreateWorkspace(ctx, acct.AccountID, "test-workspace")
	require.NotNil(t, ws, "CreateWorkspace returned nil")

	return WorkspaceFixture{
		UserID:    userID,
		AccountID: acct.AccountID,
		Workspace: ws,
	}
}

func CreateWorkspaceWithTreeFixture(t testing.TB, ctx context.Context, store *Store, userID string) WorkspaceFixture {
	t.Helper()

	fixture := CreateUserWorkspaceFixture(t, ctx, store, userID)
	tree, err := store.GetOrCreateTree(ctx, fixture.Workspace.WorkspaceID)
	require.NoError(t, err, "GetOrCreateTree")
	fixture.Tree = tree
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

func CreateWorkspaceWithProcessingJobFixture(t testing.TB, ctx context.Context, store *Store, userID string, jobType treev1.JobType) WorkspaceFixture {
	t.Helper()

	fixture := CreateWorkspaceWithTreeFixture(t, ctx, store, userID)
	doc, _, _ := store.CreateDocument(ctx, fixture.Workspace.WorkspaceID, userID, "f.pdf", "application/pdf", 100)
	require.NotNil(t, doc, "CreateDocument returned nil")
	job := store.CreateProcessingJob(ctx, doc.DocumentID, fixture.Tree.TreeID, jobType)
	require.NotNil(t, job, "CreateProcessingJob returned nil")
	fixture.Document = doc
	fixture.Job = job
	return fixture
}
