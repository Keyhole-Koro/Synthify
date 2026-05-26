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
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, nil, nil, nil, discardLogger())

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
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, nil, nil, nil, discardLogger())

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
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, nil, nil, nil, discardLogger())

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
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, nil, nil, nil, discardLogger())

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
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, metadata, nil, nil, discardLogger())
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
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, metadata, nil, nil, discardLogger())
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

func TestConfirmUploadConfirmsUploadedObjectSize(t *testing.T) {
	ctx := context.Background()
	store := mock.NewStore()
	account, err := store.GetOrCreateAccount(ctx, "owner")
	require.NoError(t, err)
	ws, err := store.CreateWorkspace(ctx, account.AccountID, "docs")
	require.NoError(t, err)
	metadata := &fakeObjectMetadata{size: 128}
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, metadata, nil, nil, discardLogger())
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
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, metadata, nil, nil, discardLogger())
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
	svc := NewDocumentService(store, store, store, store, store, store, documentSourceURL, metadata, nil, nil, discardLogger())
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

func documentSourceURL(workspaceID, documentID string) string {
	return "https://storage.example/" + workspaceID + "/" + documentID
}

type fakeObjectMetadata struct {
	size int64
}

func (f *fakeObjectMetadata) GetObjectMetadata(ctx context.Context, workspaceID, documentID string) (*domain.ObjectMetadata, error) {
	return &domain.ObjectMetadata{Size: f.size, ContentType: "application/pdf"}, nil
}
