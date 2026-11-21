package postgres

// sqlmock tests for the document deletion SQL: statement order and the
// not-found path. The SQL itself (cascades, correlated subqueries) is checked
// against CockroachDB in data_deletion_db_test.go.

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/synthify/backend/apps/api/internal/domain"
)

func TestDeleteDocumentRows_StatementOrder(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT workspace_id FROM documents WHERE document_id = $1 FOR UPDATE")).
		WithArgs("doc_1").
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow("ws_1"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM tree_items")).
		WithArgs("ws_1", "doc_1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("parent"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, COALESCE(parent_id, '') FROM tree_items WHERE workspace_id = $1")).
		WithArgs("ws_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).
			AddRow("root", "").
			AddRow("parent", "root").
			AddRow("child", "parent"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE tree_items SET parent_id = $2, updated_at = $3 WHERE id = $1")).
		WithArgs("child", sql.NullString{String: "root", Valid: true}, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM tree_items")).
		WithArgs("ws_1", "doc_1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("SET cross_document")).
		WithArgs("ws_1", "doc_1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("FROM upload_reservations")).
		WithArgs("doc_1").
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "actual_size_bytes"}).AddRow("acc_1", int64(300)))
	mock.ExpectExec(regexp.QuoteMeta("SET storage_used_bytes = GREATEST(storage_used_bytes - $2, 0)")).
		WithArgs("acc_1", int64(300), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM documents WHERE document_id = $1")).
		WithArgs("doc_1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deleted, err := store.DeleteDocumentRows(context.Background(), "doc_1")

	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteDocumentRows_MissingDocument(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("FOR UPDATE")).
		WithArgs("doc_missing").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err := store.DeleteDocumentRows(context.Background(), "doc_missing")

	assert.ErrorIs(t, err, domain.ErrNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Nodes a person curated or locked are protected from automated changes, so
// deleting the document they came from leaves them, and their children, in
// place. Only the unprotected node goes, and only its child moves up.
func TestDeleteDocumentRows_KeepsCuratedAndLockedNodes(t *testing.T) {
	store, mock, cleanup := newStoreWithMock(t)
	defer cleanup()
	protectedFilter := regexp.QuoteMeta("governance_state NOT IN ('human_curated', 'locked')")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT workspace_id FROM documents WHERE document_id = $1 FOR UPDATE")).
		WithArgs("doc_1").
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow("ws_1"))
	// The database answers with the unprotected node only: the filter keeps
	// "curated" and "locked" out of the set to delete.
	mock.ExpectQuery(`(?s)SELECT id FROM tree_items.*`+protectedFilter).
		WithArgs("ws_1", "doc_1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("plain"))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, COALESCE(parent_id, '') FROM tree_items WHERE workspace_id = $1")).
		WithArgs("ws_1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id"}).
			AddRow("curated", "").
			AddRow("curated-child", "curated").
			AddRow("locked", "").
			AddRow("locked-child", "locked").
			AddRow("plain", "").
			AddRow("plain-child", "plain"))
	// Only the child of the deleted node moves; the protected nodes' children
	// keep their parents.
	mock.ExpectExec(regexp.QuoteMeta("UPDATE tree_items SET parent_id = $2, updated_at = $3 WHERE id = $1")).
		WithArgs("plain-child", sql.NullString{}, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)DELETE FROM tree_items.*`+protectedFilter).
		WithArgs("ws_1", "doc_1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("SET cross_document")).
		WithArgs("ws_1", "doc_1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery(regexp.QuoteMeta("FROM upload_reservations")).
		WithArgs("doc_1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM documents WHERE document_id = $1")).
		WithArgs("doc_1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	deleted, err := store.DeleteDocumentRows(context.Background(), "doc_1")

	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}
