package postgres

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	appv1 "github.com/synthify/backend/internal/gen/synthify/app/v1"
)

func TestCreateProcessingJobCreatesJobBeforeCapability(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	now := time.Now().UTC()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT document_id, workspace_id, uploaded_by, filename, mime_type, file_size, created_at FROM documents WHERE document_id = $1")).
		WithArgs("doc_1").
		WillReturnRows(sqlmock.NewRows([]string{
			"document_id",
			"workspace_id",
			"uploaded_by",
			"filename",
			"mime_type",
			"file_size",
			"created_at",
		}).AddRow("doc_1", "ws_1", "user_1", "paper.pdf", "application/pdf", int64(128), now))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO document_processing_jobs")).
		WithArgs(
			sqlmock.AnyArg(),
			"doc_1",
			"ws_1",
			"process_document",
			"queued",
			"",
			"",
			"",
			"user_1",
			sqlmock.AnyArg(),
			"",
			"",
			"",
			int32(0),
			"",
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO job_capabilities")).
		WithArgs(
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"ws_1",
			`["doc_1"]`,
			"null",
			sqlmock.AnyArg(),
			int32(128),
			int32(0),
			int32(4096),
			int32(64),
			int32(512),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	job, err := store.CreateProcessingJob(ctx, "doc_1", "ws_1", "user_1", appv1.JobType_JOB_TYPE_PROCESS_DOCUMENT)
	require.NoError(t, err)
	require.NotEmpty(t, job.JobID)
	require.NotEmpty(t, job.CapabilityID)

	require.NoError(t, mock.ExpectationsWereMet())
}
