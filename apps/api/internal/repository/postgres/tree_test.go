package postgres

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/synthify/backend/apps/api/internal/domain"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindPaths_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	// No expectations set → any query returns an error → FindPaths returns error.
	_, _, err = store.FindPaths(context.Background(), "nonexistent_tree", "n1", "n2", 4, 3)
	assert.Error(t, err, "expected error on DB error, got nil")
}

func TestGetTree_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	_, err = store.GetTree(context.Background(), "ws_1")
	assert.Error(t, err, "expected error on DB failure, got nil")
}

// treeItemColumns mirrors ListItemsByWorkspace's select list.
var treeItemColumns = []string{
	"id", "workspace_id", "parent_id", "title", "level", "description", "content",
	"override_css", "created_by", "governance_state", "cross_document", "created_at",
}

func treeItemRow(id, parentID string) []driver.Value {
	var parent driver.Value
	if parentID != "" {
		parent = parentID
	}
	return []driver.Value{id, "ws_1", parent, id, int32(0), "", "", "", "", "system_generated", false, time.Now()}
}

// ChildIDs are filled in with a single query for the whole workspace. Anything
// per-item would issue queries sqlmock has not been told to expect, which
// surfaces as an error rather than as a quietly slower endpoint — which is how
// the 1+N version went unnoticed.
func TestGetTreeByWorkspace_PopulatesChildIDsWithOneQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	mock.ExpectQuery("FROM tree_items").WillReturnRows(
		sqlmock.NewRows(treeItemColumns).
			AddRow(treeItemRow("root", "")...).
			AddRow(treeItemRow("a", "root")...).
			AddRow(treeItemRow("b", "root")...).
			AddRow(treeItemRow("leaf", "a")...),
	)
	mock.ExpectQuery("SELECT parent_id, id").WillReturnRows(
		sqlmock.NewRows([]string{"parent_id", "id"}).
			AddRow("root", "a").
			AddRow("root", "b").
			AddRow("a", "leaf"),
	)

	store := &Store{db: db}
	items, err := store.GetTreeByWorkspace(context.Background(), "ws_1")
	require.NoError(t, err)

	byID := make(map[string]*domain.Item, len(items))
	for _, item := range items {
		byID[item.ItemID] = item
	}
	assert.Equal(t, []string{"a", "b"}, byID["root"].ChildIDs, "sibling order follows the query's ORDER BY")
	assert.Equal(t, []string{"leaf"}, byID["a"].ChildIDs)
	assert.Equal(t, []string{}, byID["b"].ChildIDs, "a childless node gets an empty, non-nil slice")
	assert.Equal(t, []string{}, byID["leaf"].ChildIDs)

	require.NoError(t, mock.ExpectationsWereMet(), "expected exactly one child-id query")
}

// A subtree listing holds only part of the workspace, so the edge query returns
// parents that are not in the result set. Those must be dropped, not panic.
func TestGetTreeByWorkspace_IgnoresEdgesForItemsNotListed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	mock.ExpectQuery("FROM tree_items").WillReturnRows(
		sqlmock.NewRows(treeItemColumns).AddRow(treeItemRow("root", "")...),
	)
	mock.ExpectQuery("SELECT parent_id, id").WillReturnRows(
		sqlmock.NewRows([]string{"parent_id", "id"}).
			AddRow("root", "a").
			AddRow("elsewhere", "b"),
	)

	store := &Store{db: db}
	items, err := store.GetTreeByWorkspace(context.Background(), "ws_1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, []string{"a"}, items[0].ChildIDs)
	require.NoError(t, mock.ExpectationsWereMet())
}

// The per-item version swallowed query errors; a failure of the single query
// would silently flatten the whole tree, so it must propagate.
func TestGetTreeByWorkspace_ChildIDQueryError_Propagates(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	mock.ExpectQuery("FROM tree_items").WillReturnRows(
		sqlmock.NewRows(treeItemColumns).AddRow(treeItemRow("root", "")...),
	)
	mock.ExpectQuery("SELECT parent_id, id").WillReturnError(errors.New("connection reset"))

	store := &Store{db: db}
	_, err = store.GetTreeByWorkspace(context.Background(), "ws_1")
	assert.Error(t, err)
}
