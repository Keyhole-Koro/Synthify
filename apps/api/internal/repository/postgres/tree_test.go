package postgres

import (
	"context"
	"testing"

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

func TestGetOrCreateTree_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	_, err = store.GetOrCreateTree(context.Background(), "ws_1")
	assert.Error(t, err, "expected error on DB failure, got nil")
}
