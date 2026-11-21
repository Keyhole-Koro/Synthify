package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

// recordingPrefixDeleter records the prefixes it is asked to delete and fails
// on the ones listed in failOn.
type recordingPrefixDeleter struct {
	mu       sync.Mutex
	prefixes []string
	failOn   map[string]bool
}

func (d *recordingPrefixDeleter) DeleteObjectsWithPrefix(_ context.Context, prefix string) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failOn[prefix] {
		return 0, errors.New("storage: permission denied")
	}
	d.prefixes = append(d.prefixes, prefix)
	return 1, nil
}

type documentDeletionFixture struct {
	store   *mock.Store
	objects *recordingPrefixDeleter
	svc     *DocumentService
	ws      string
}

// newDocumentDeletionFixture makes a workspace owned by "owner" with an editor
// ("editor") and a viewer ("viewer").
func newDocumentDeletionFixture(t *testing.T) documentDeletionFixture {
	t.Helper()
	ctx := context.Background()
	store := mock.NewStore()
	fixture := mock.CreateUserWorkspaceFixture(t, ctx, store, "owner")
	ws := fixture.Workspace.WorkspaceID
	require.NoError(t, store.UpsertWorkspaceMember(ctx, ws, "editor", domain.WorkspaceRoleEditor, "owner"))
	require.NoError(t, store.UpsertWorkspaceMember(ctx, ws, "viewer", domain.WorkspaceRoleViewer, "owner"))
	objects := &recordingPrefixDeleter{}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		Deletion:         store,
		ObjectPrefixes:   objects,
		Logger:           discardLogger(),
	})
	return documentDeletionFixture{store: store, objects: objects, svc: svc, ws: ws}
}

// uploadedDocument creates a document whose upload is confirmed and whose
// processing job finished.
func (f documentDeletionFixture) uploadedDocument(t *testing.T, uploadedBy, filename string, size int64) (*domain.Document, *domain.DocumentProcessingJob) {
	t.Helper()
	ctx := context.Background()
	doc, _, err := f.store.CreateDocument(ctx, f.ws, uploadedBy, filename, "application/pdf", size)
	require.NoError(t, err)
	require.NoError(t, f.store.ConfirmDocumentUpload(ctx, doc.DocumentID, size))
	job, err := f.store.CreateProcessingJob(ctx, doc.DocumentID, f.ws, uploadedBy, appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)
	require.NoError(t, f.store.CompleteProcessingJob(ctx, job.JobID))
	return doc, job
}

func (f documentDeletionFixture) node(t *testing.T, label, parentID string, documentIDs ...string) string {
	t.Helper()
	ctx := context.Background()
	item, err := f.store.CreateItem(ctx, f.ws, label, "", parentID, "worker")
	require.NoError(t, err)
	for _, documentID := range documentIDs {
		require.NoError(t, f.store.UpsertItemSource(ctx, item.ItemID, documentID, "file", "", "", 0))
	}
	return item.ItemID
}

func TestDeleteDocument_RemovesFilesRowsAndNodesMadeOnlyFromIt(t *testing.T) {
	ctx := context.Background()
	f := newDocumentDeletionFixture(t)
	doc, job := f.uploadedDocument(t, "owner", "a.pdf", 300)
	other, _ := f.uploadedDocument(t, "owner", "b.pdf", 200)
	only := f.node(t, "only-a", "", doc.DocumentID)
	child := f.node(t, "child-of-only-a", only, other.DocumentID)
	shared := f.node(t, "shared", "", doc.DocumentID, other.DocumentID)

	err := f.svc.DeleteDocument(ctx, doc.DocumentID, "owner")

	require.NoError(t, err)
	assert.Equal(t, []string{
		f.ws + "/" + doc.DocumentID + "/",
		".cache/v1/" + doc.DocumentID + "/",
		".checkpoints/" + job.JobID + "/",
	}, f.objects.prefixes)
	_, err = f.store.GetDocument(ctx, doc.DocumentID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = f.store.GetItem(ctx, only)
	assert.ErrorIs(t, err, domain.ErrNotFound, "a node made only from the document goes with it")
	kept, err := f.store.GetItem(ctx, child)
	require.NoError(t, err)
	assert.Empty(t, kept.ParentID, "its child cites another document, so it moves up to the root")
	sources, err := f.store.ListItemSources(ctx, shared)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, other.DocumentID, sources[0].DocumentID)
	account, err := f.store.GetAccountByUser(ctx, "owner")
	require.NoError(t, err)
	assert.EqualValues(t, 200, account.StorageUsedBytes)
}

func TestDeleteDocument_Authorization(t *testing.T) {
	cases := []struct {
		name       string
		uploadedBy string
		caller     string
		allowed    bool
	}{
		{"owner deletes an editor's upload", "editor", "owner", true},
		{"editor deletes their own upload", "editor", "editor", true},
		{"editor cannot delete the owner's upload", "owner", "editor", false},
		{"viewer cannot delete their former upload", "viewer", "viewer", false},
		{"outsider cannot delete", "owner", "stranger", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDocumentDeletionFixture(t)
			doc, _ := f.uploadedDocument(t, tc.uploadedBy, "a.pdf", 10)

			err := f.svc.DeleteDocument(context.Background(), doc.DocumentID, tc.caller)

			if tc.allowed {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, domain.ErrForbidden)
			assert.Empty(t, f.objects.prefixes, "nothing is deleted for a refused caller")
			_, err = f.store.GetDocument(context.Background(), doc.DocumentID)
			assert.NoError(t, err)
		})
	}
}

func TestDeleteDocument_MissingDocumentIsForbidden(t *testing.T) {
	f := newDocumentDeletionFixture(t)

	err := f.svc.DeleteDocument(context.Background(), "doc-missing", "owner")

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestDeleteDocument_RefusesWhileUploadIsOpen(t *testing.T) {
	ctx := context.Background()
	f := newDocumentDeletionFixture(t)
	doc, _, err := f.store.CreateDocument(ctx, f.ws, "owner", "a.pdf", "application/pdf", 10)
	require.NoError(t, err)

	err = f.svc.DeleteDocument(ctx, doc.DocumentID, "owner")

	assert.ErrorIs(t, err, domain.ErrDocumentBusy)
	assert.Empty(t, f.objects.prefixes)
}

func TestDeleteDocument_RefusesWhileAJobIsRunning(t *testing.T) {
	ctx := context.Background()
	f := newDocumentDeletionFixture(t)
	doc, _ := f.uploadedDocument(t, "owner", "a.pdf", 10)
	running, err := f.store.CreateProcessingJob(ctx, doc.DocumentID, f.ws, "owner", appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT)
	require.NoError(t, err)
	require.NoError(t, f.store.MarkProcessingJobRunning(ctx, running.JobID))

	err = f.svc.DeleteDocument(ctx, doc.DocumentID, "owner")

	assert.ErrorIs(t, err, domain.ErrDocumentBusy)
	assert.Empty(t, f.objects.prefixes)
}

// A job that stopped reporting long ago is stuck, not running: it must not
// make its document undeletable.
func TestDeleteDocument_StuckJobDoesNotBlock(t *testing.T) {
	ctx := context.Background()
	f := newDocumentDeletionFixture(t)
	doc, _ := f.uploadedDocument(t, "owner", "a.pdf", 10)
	stuck, err := f.store.CreateProcessingJob(ctx, doc.DocumentID, f.ws, "owner", appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT)
	require.NoError(t, err)
	stuck.Status = appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_RUNNING
	stuck.UpdatedAt = time.Now().Add(-2 * stuckJobMinAge).UTC().Format(time.RFC3339)

	err = f.svc.DeleteDocument(ctx, doc.DocumentID, "owner")

	require.NoError(t, err)
}

// If the stored files cannot be deleted, the rows stay: the document stays
// listed and the user can retry, instead of the files being orphaned.
func TestDeleteDocument_KeepsRowsWhenStorageDeletionFails(t *testing.T) {
	ctx := context.Background()
	f := newDocumentDeletionFixture(t)
	doc, _ := f.uploadedDocument(t, "owner", "a.pdf", 10)
	f.objects.failOn = map[string]bool{".cache/v1/" + doc.DocumentID + "/": true}

	err := f.svc.DeleteDocument(ctx, doc.DocumentID, "owner")

	require.Error(t, err)
	_, err = f.store.GetDocument(ctx, doc.DocumentID)
	assert.NoError(t, err)
}

func TestDeleteDocument_UnavailableWithoutDeletionDependencies(t *testing.T) {
	store := mock.NewStore()
	svc := NewDocumentService(DocumentServiceDeps{Repo: store, Jobs: store, LifecycleRepo: store, Workspaces: store})

	err := svc.DeleteDocument(context.Background(), "doc", "owner")

	assert.ErrorIs(t, err, domain.ErrNotImplemented)
}
