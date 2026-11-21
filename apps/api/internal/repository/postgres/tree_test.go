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

// treeTestItem is the id and workspace of a tree_items row in these tests.
type treeTestItem struct {
	id          string
	workspaceID string
}

var treeItemColumns = []string{
	"id", "workspace_id", "parent_id", "title", "level", "description", "content",
	"override_css", "created_by", "governance_state", "cross_document", "created_at",
}

// treeItemRows builds the result of GetItem / ListItemsByWorkspace.
func treeItemRows(items ...treeTestItem) *sqlmock.Rows {
	rows := sqlmock.NewRows(treeItemColumns)
	for _, item := range items {
		rows.AddRow(item.id, item.workspaceID, nil, item.id, 0, "", "", "", "user_1", "system_generated", false, time.Now())
	}
	return rows
}

// childItemRows builds the result of ListChildItems, which adds has_children.
func childItemRows(items ...treeTestItem) *sqlmock.Rows {
	rows := sqlmock.NewRows(append(append([]string{}, treeItemColumns...), "has_children"))
	for _, item := range items {
		rows.AddRow(item.id, item.workspaceID, "parent", item.id, 0, "", "", "", "user_1", "system_generated", false, time.Now(), false)
	}
	return rows
}

const (
	getItemQuery          = "FROM tree_items\nWHERE id = $1"
	listChildItemsQuery   = "WHERE tree_items.parent_id = $1"
	listItemsByWorkspaceQ = "WHERE workspace_id = $1\nORDER BY created_at ASC"
)

// ListChildItems matches parent_id alone, and an item could be created under a
// parent from another workspace. The tree reads must leave such children out.
func TestGetSubtree_ReturnsOnlyChildrenInTheRootWorkspace(t *testing.T) {
	tests := []struct {
		name     string
		children []treeTestItem
		want     []string
	}{
		{
			name:     "children in the same workspace",
			children: []treeTestItem{{"child-1", "ws-a"}, {"child-2", "ws-a"}},
			want:     []string{"root", "child-1", "child-2"},
		},
		{
			name:     "a child from another workspace",
			children: []treeTestItem{{"child-1", "ws-a"}, {"foreign", "ws-b"}},
			want:     []string{"root", "child-1"},
		},
		{
			name:     "only children from another workspace",
			children: []treeTestItem{{"foreign", "ws-b"}},
			want:     []string{"root"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			store := &Store{db: db}

			mock.ExpectQuery(regexp.QuoteMeta(getItemQuery)).
				WithArgs("root").
				WillReturnRows(treeItemRows(treeTestItem{"root", "ws-a"}))
			mock.ExpectQuery(regexp.QuoteMeta(listChildItemsQuery)).
				WithArgs("root").
				WillReturnRows(childItemRows(tt.children...))
			// populateChildIDs then lists the children of every returned item.
			for _, id := range tt.want {
				mock.ExpectQuery(regexp.QuoteMeta(listChildItemsQuery)).
					WithArgs(id).
					WillReturnRows(childItemRows())
			}

			items, err := store.GetSubtree(context.Background(), "root", 3)
			require.NoError(t, err)
			got := make([]string, 0, len(items))
			for _, item := range items {
				got = append(got, item.ItemID)
			}
			assert.Equal(t, tt.want, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetTreeByWorkspace_ChildIDsLeaveOutOtherWorkspaces(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	store := &Store{db: db}

	mock.ExpectQuery(regexp.QuoteMeta(listItemsByWorkspaceQ)).
		WithArgs("ws-a").
		WillReturnRows(treeItemRows(treeTestItem{"root", "ws-a"}))
	mock.ExpectQuery(regexp.QuoteMeta(listChildItemsQuery)).
		WithArgs("root").
		WillReturnRows(childItemRows(treeTestItem{"child", "ws-a"}, treeTestItem{"foreign", "ws-b"}))

	items, err := store.GetTreeByWorkspace(context.Background(), "ws-a")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, []string{"child"}, items[0].ChildIDs)
	require.NoError(t, mock.ExpectationsWereMet())
}
