package service

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
	"github.com/synthify/backend/apps/api/internal/repository/mock"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestCreateDocumentRejectsOversizedFile(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		Logger:           discardLogger(),
	})

	doc, uploadURL, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "huge.pdf", "application/pdf", account.MaxFileSizeBytes+1)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrFileTooLarge)
	assert.Nil(t, doc)
	assert.Empty(t, uploadURL)
}

func TestCreateDocumentReservesQuotaUntilConfirmation(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	account.StorageQuotaBytes = 200
	account.MaxFileSizeBytes = 200
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		Logger:           discardLogger(),
	})

	first, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "first.pdf", "application/pdf", 150)
	require.NoError(t, err)
	require.NotNil(t, first)
	second, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "second.pdf", "application/pdf", 100)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrStorageQuotaExceeded)
	assert.Nil(t, second)
}

func TestCreateDocumentAllowsExactQuotaLimit(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	account.StorageQuotaBytes = 128
	account.MaxFileSizeBytes = 128
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		Logger:           discardLogger(),
	})

	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "exact.pdf", "application/pdf", 128)

	require.NoError(t, err)
	require.NotNil(t, doc)
}

func TestExpireUploadReservationsReleasesReservedQuota(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	account.StorageQuotaBytes = 200
	account.MaxFileSizeBytes = 200
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		Logger:           discardLogger(),
	})

	expired, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "expired.pdf", "application/pdf", 150)
	require.NoError(t, err)
	expiredCount, err := svc.ExpireUploadReservations(ctx, time.Now().Add(16*time.Minute))
	require.NoError(t, err)
	replacement, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "replacement.pdf", "application/pdf", 100)

	assert.Equal(t, int64(1), expiredCount)
	require.NoError(t, err)
	require.NotNil(t, replacement)
	err = store.ConfirmDocumentUpload(ctx, expired.DocumentID, 150)
	assert.ErrorIs(t, err, domain.ErrUploadNotConfirmed)
}

func TestStartProcessingConfirmsUploadedObjectSize(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "paper.pdf", "application/pdf", 128)
	require.NoError(t, err)

	job, err := svc.StartProcessing(ctx, doc.DocumentID, "owner", false)

	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, ws.WorkspaceID, job.WorkspaceID)
	assert.Equal(t, "owner", job.RequestedBy)
	updated, err := store.GetAccount(ctx, account.AccountID)
	require.NoError(t, err)
	assert.Equal(t, int64(128), updated.StorageUsedBytes)
}

func TestStartProcessingRejectsSizeMismatch(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 256}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "paper.pdf", "application/pdf", 128)
	require.NoError(t, err)

	job, err := svc.StartProcessing(ctx, doc.DocumentID, "owner", false)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUploadSizeMismatch)
	assert.Nil(t, job)
	updated, err := store.GetAccount(ctx, account.AccountID)
	require.NoError(t, err)
	assert.Zero(t, updated.StorageUsedBytes)
}

func TestStartProcessingRejectsUnsupportedUploadedContentType(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128, contentType: "application/x-msdownload"}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "payload.dat", "text/plain", 128)
	require.NoError(t, err)

	job, err := svc.StartProcessing(ctx, doc.DocumentID, "owner", false)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnsupportedDocumentType)
	assert.Nil(t, job)
}

func TestConfirmUploadRejectsUnsupportedUploadedContentType(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128, contentType: "application/x-msdownload"}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "payload.dat", "text/plain", 128)
	require.NoError(t, err)

	confirmed, err := svc.ConfirmUpload(ctx, doc.DocumentID, "owner")

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnsupportedDocumentType)
	assert.Nil(t, confirmed)
}

func TestConfirmUploadAllowsOctetStreamForReadableExtension(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128, contentType: "application/octet-stream"}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "notes.md", "application/octet-stream", 128)
	require.NoError(t, err)

	confirmed, err := svc.ConfirmUpload(ctx, doc.DocumentID, "owner")

	require.NoError(t, err)
	require.NotNil(t, confirmed)
}

func TestConfirmUploadConfirmsUploadedObjectSize(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "paper.pdf", "application/pdf", 128)
	require.NoError(t, err)

	confirmed, err := svc.ConfirmUpload(ctx, doc.DocumentID, "owner")

	require.NoError(t, err)
	require.NotNil(t, confirmed)
	updated, err := store.GetAccount(ctx, account.AccountID)
	require.NoError(t, err)
	assert.Equal(t, int64(128), updated.StorageUsedBytes)
}

func TestConfirmUploadRejectsSizeMismatch(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 256}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "paper.pdf", "application/pdf", 128)
	require.NoError(t, err)

	confirmed, err := svc.ConfirmUpload(ctx, doc.DocumentID, "owner")

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUploadSizeMismatch)
	assert.Nil(t, confirmed)
	updated, err := store.GetAccount(ctx, account.AccountID)
	require.NoError(t, err)
	assert.Zero(t, updated.StorageUsedBytes)
}

func TestStartProcessingRespectsForceReprocess(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "paper.pdf", "application/pdf", 128)
	require.NoError(t, err)

	// First time
	job1, err := svc.StartProcessing(ctx, doc.DocumentID, "owner", false)
	require.NoError(t, err)
	require.NotNil(t, job1)
	job1.Status = appv1.JobLifecycleState_JOB_LIFECYCLE_STATE_SUCCEEDED

	// Second time without force - should return job1
	job2, err := svc.StartProcessing(ctx, doc.DocumentID, "owner", false)
	require.NoError(t, err)
	assert.Equal(t, job1.JobID, job2.JobID, "should return existing completed job")

	// Third time with force - should return a new job
	job3, err := svc.StartProcessing(ctx, doc.DocumentID, "owner", true)
	require.NoError(t, err)
	assert.NotEqual(t, job1.JobID, job3.JobID, "should create new job when forced")
	assert.Equal(t, appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT, job3.JobType)
}

func TestAutoResumeFailedJobsRetriesLatestFailedJobOnce(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128}
	dispatcher := &fakeDispatcher{}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Dispatcher:       dispatcher,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "paper.pdf", "application/pdf", 128)
	require.NoError(t, err)
	failed, err := store.CreateProcessingJob(ctx, doc.DocumentID, ws.WorkspaceID, "owner", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)
	require.NoError(t, store.FailProcessingJob(ctx, failed.JobID, "transient failure"))

	resumed, err := svc.AutoResumeFailedJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 1, resumed)
	assert.Equal(t, 1, dispatcher.executeCalls)
	latest, err := store.GetLatestProcessingJob(ctx, doc.DocumentID)
	require.NoError(t, err)
	assert.NotEqual(t, failed.JobID, latest.JobID)
	assert.Equal(t, 1, latest.RetryCount)
}

func TestAutoResumeFailedJobsSkipsAlreadyRetriedLatestFailure(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128}
	dispatcher := &fakeDispatcher{}
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		ObjectMetadata:   metadata,
		Dispatcher:       dispatcher,
		Logger:           discardLogger(),
	})
	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "owner", "paper.pdf", "application/pdf", 128)
	require.NoError(t, err)
	failed, err := store.CreateProcessingJob(ctx, doc.DocumentID, ws.WorkspaceID, "owner", appv1.JobType_JOB_TYPE_REPROCESS_DOCUMENT)
	require.NoError(t, err)
	require.NoError(t, store.FailProcessingJob(ctx, failed.JobID, "still failing"))
	require.NoError(t, store.SetProcessingJobRetryCount(ctx, failed.JobID, 1))

	resumed, err := svc.AutoResumeFailedJobs(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, resumed)
	assert.Equal(t, 0, dispatcher.executeCalls)
}

func documentSourceURL(workspaceID, documentID string) string {
	return "https://storage.example/" + workspaceID + "/" + documentID
}

type fakeObjectMetadata struct {
	size        int64
	contentType string
}

func (f *fakeObjectMetadata) GetObjectMetadata(ctx context.Context, workspaceID, documentID string) (*domain.ObjectMetadata, error) {
	contentType := f.contentType
	if contentType == "" {
		contentType = "application/pdf"
	}
	return &domain.ObjectMetadata{Size: f.size, ContentType: contentType}, nil
}

type fakeDispatcher struct {
	generateCalls int
	executeCalls  int
	executeErr    error
}

func TestCreateDocument_Viewer_ReturnsForbidden(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, ws.WorkspaceID, "viewer", domain.WorkspaceRoleViewer, "owner"))
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		Logger:           discardLogger(),
	})

	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "viewer", "f.pdf", "application/pdf", 10)
	assert.ErrorIs(t, err, domain.ErrForbidden, "viewer cannot create documents")
	assert.Nil(t, doc)
}

func TestCreateDocument_Editor_Succeeds(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	account.StorageQuotaBytes = 1000
	account.MaxFileSizeBytes = 1000
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	require.NoError(t, store.UpsertWorkspaceMember(ctx, ws.WorkspaceID, "editor", domain.WorkspaceRoleEditor, "owner"))
	svc := NewDocumentService(DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: documentSourceURL,
		Logger:           discardLogger(),
	})

	doc, _, err := svc.CreateDocument(ctx, ws.WorkspaceID, "editor", "f.pdf", "application/pdf", 10)
	require.NoError(t, err, "editor can create documents")
	assert.NotNil(t, doc)
}

func (d *fakeDispatcher) GenerateExecutionPlan(ctx context.Context, req domain.ExecutePlanRequest) error {
	d.generateCalls++
	return nil
}

func (d *fakeDispatcher) ExecuteApprovedPlan(ctx context.Context, req domain.ExecutePlanRequest) error {
	d.executeCalls++
	if d.executeErr != nil {
		return d.executeErr
	}
	return nil
}
