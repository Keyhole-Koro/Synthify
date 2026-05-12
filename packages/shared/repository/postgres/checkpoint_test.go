package postgres

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertStageRunning_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New should not fail")
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	jobID := "job_123"
	stage := "briefing"

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO job_stage_checkpoints")).
		WithArgs(jobID, stage, "running", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.UpsertStageRunning(ctx, jobID, stage)
	require.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestMarkStageSucceeded_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New should not fail")
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	jobID := "job_123"
	stage := "briefing"
	gcsRef := "workspaces/ws_1/jobs/job_123/briefing.json"

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO job_stage_checkpoints")).
		WithArgs(jobID, stage, "succeeded", gcsRef, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = store.MarkStageSucceeded(ctx, jobID, stage, gcsRef)
	require.NoError(t, err)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

func TestListStageCheckpoints_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New should not fail")
	defer db.Close()

	store := &Store{db: db}
	ctx := context.Background()
	jobID := "job_123"

	rows := sqlmock.NewRows([]string{"job_id", "stage", "status", "gcs_ref", "updated_at"}).
		AddRow(jobID, "briefing", "succeeded", "ref_1", time.Now()).
		AddRow(jobID, "synthesis", "running", "", time.Now())

	mock.ExpectQuery(regexp.QuoteMeta("SELECT job_id, stage, status, gcs_ref, updated_at FROM job_stage_checkpoints")).
		WithArgs(jobID).
		WillReturnRows(rows)

	checkpoints, err := store.ListStageCheckpoints(ctx, jobID)
	require.NoError(t, err)

	require.Len(t, checkpoints, 2)

	assert.Equal(t, "briefing", checkpoints[0].Stage)
	assert.Equal(t, "succeeded", checkpoints[0].Status)

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}
