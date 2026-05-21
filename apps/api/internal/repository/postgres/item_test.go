package postgres

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateItem_DBError_ReturnsNil(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	// No expectations set → Begin or any query returns an error.
	item := store.CreateItem(context.Background(), "tree_1", "New Item", "Description", "", "user_1")
	assert.Nil(t, item, "expected nil on DB error, got item")
}

func TestGetItem_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	_, err = store.GetItem(context.Background(), "nonexistent_item")
	assert.Error(t, err, "expected error on DB error, got nil")
}
