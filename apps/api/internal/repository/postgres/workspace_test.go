package postgres

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateWorkspace_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	// No expectations set → any query returns an error.
	ws, err := store.CreateWorkspace(context.Background(), "acc_1", "Test Workspace")
	assert.Error(t, err, "expected error on DB error, got nil")
	assert.Nil(t, ws, "expected nil workspace on DB error")
}

func TestGetWorkspace_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	_, err = store.GetWorkspace(context.Background(), "nonexistent_id")
	assert.Error(t, err, "expected error on DB error, got nil")
}

func TestIsWorkspaceAccessible_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	ok, err := store.IsWorkspaceAccessible(context.Background(), "ws_1", "user_1")
	assert.Error(t, err, "expected error on DB error, got nil")
	assert.False(t, ok, "expected ok=false on DB error")
}
