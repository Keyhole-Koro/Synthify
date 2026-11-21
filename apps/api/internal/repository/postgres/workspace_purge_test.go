package postgres

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListWorkspacesDeletedBefore_QueriesByCutoffAndLimit(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()
	cutoff := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(regexp.QuoteMeta("WHERE deleted_at IS NOT NULL AND deleted_at <= $1")).
		WithArgs(cutoff, 10).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow("ws_1").AddRow("ws_2"))

	ids, err := store.ListWorkspacesDeletedBefore(context.Background(), cutoff, 10)

	require.NoError(t, err)
	assert.Equal(t, []string{"ws_1", "ws_2"}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPurgeWorkspaceRows_StatementOrder(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT account_id FROM workspaces WHERE workspace_id = $1 AND deleted_at IS NOT NULL FOR UPDATE")).
		WithArgs("ws_1").
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow("acc_1"))
	mock.ExpectQuery(regexp.QuoteMeta("FROM upload_reservations")).
		WithArgs("ws_1").
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "sum"}).AddRow("acc_1", int64(500)))
	mock.ExpectExec(regexp.QuoteMeta("SET storage_used_bytes = GREATEST(storage_used_bytes - $2, 0)")).
		WithArgs("acc_1", int64(500), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM workspaces WHERE workspace_id = $1")).
		WithArgs("ws_1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	purged, err := store.PurgeWorkspaceRows(context.Background(), "ws_1")

	require.NoError(t, err)
	assert.True(t, purged)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A workspace that is not marked deleted (or no longer exists) is never purged.
func TestPurgeWorkspaceRows_RefusesWorkspacesNotMarkedDeleted(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("deleted_at IS NOT NULL FOR UPDATE")).
		WithArgs("ws_live").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	purged, err := store.PurgeWorkspaceRows(context.Background(), "ws_live")

	require.NoError(t, err)
	assert.False(t, purged)
	require.NoError(t, mock.ExpectationsWereMet())
}
