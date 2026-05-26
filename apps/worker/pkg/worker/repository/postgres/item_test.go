package postgres

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateItem_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	// No expectations set → any query returns an error.
	item, err := store.CreateItem(context.Background(), "tree_1", "New Item", "Description", "", "user_1")
	assert.Error(t, err, "expected error on DB error, got nil")
	assert.Nil(t, item, "expected nil item on DB error")
}

func TestGetItem_DBError_ReturnsError(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err, "sqlmock.New")
	defer db.Close()

	store := &Store{db: db}

	_, err = store.GetItem(context.Background(), "nonexistent_item")
	assert.Error(t, err, "expected error on DB error, got nil")
}
