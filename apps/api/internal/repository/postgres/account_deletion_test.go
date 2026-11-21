package postgres

import (
	"context"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

// A workspace created after the purge must not be orphaned by deleting its
// account: the delete is refused instead.
func TestDeleteAccountRows_RefusesWhileAWorkspaceIsLeft(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS (SELECT 1 FROM workspaces WHERE account_id IN (")).
		WithArgs("user_1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	err := store.DeleteAccountRows(context.Background(), "user_1")

	assert.ErrorIs(t, err, domain.ErrConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteAccountRows_StatementOrder(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS (SELECT 1 FROM workspaces WHERE account_id IN (")).
		WithArgs("user_1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	for _, statement := range []string{
		"DELETE FROM workspace_members WHERE user_id = $1",
		"UPDATE workspace_members SET invited_by = '' WHERE invited_by = $1",
		"UPDATE documents SET uploaded_by = '' WHERE uploaded_by = $1",
		"UPDATE document_processing_jobs SET requested_by = '' WHERE requested_by = $1",
		"UPDATE tree_items SET created_by = '' WHERE created_by = $1",
		"DELETE FROM accounts WHERE account_id IN (",
		"DELETE FROM account_users WHERE user_id = $1",
		"DELETE FROM users WHERE user_id = $1",
	} {
		mock.ExpectExec(regexp.QuoteMeta(statement)).WithArgs("user_1").WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	require.NoError(t, store.DeleteAccountRows(context.Background(), "user_1"))
	require.NoError(t, mock.ExpectationsWereMet())
}
