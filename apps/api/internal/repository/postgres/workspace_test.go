package postgres

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateWorkspace_DBError_ReturnsNil(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	// No expectations set → any query returns an error.
	ws := store.CreateWorkspace(context.Background(), "acc_1", "Test Workspace")
	assert.Nil(t, ws, "expected nil on DB error, got workspace")
}

func TestGetWorkspace_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	_, err = store.GetWorkspace(context.Background(), "nonexistent_id")
	assert.Error(t, err, "expected error on DB error, got nil")
}

func TestIsWorkspaceAccessible_DBError_ReturnsFalse(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	assert.False(t, store.IsWorkspaceAccessible(context.Background(), "ws_1", "user_1"), "expected false on DB error, got true")
}
