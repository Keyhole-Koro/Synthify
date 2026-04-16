package postgres

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestCreateNode_DBError_ReturnsNil(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &Store{db: db}

	// No expectations set → Begin or any query returns an error.
	node := store.CreateNode("graph_1", "New Node", "Description", "", "user_1")
	if node != nil {
		t.Fatal("expected nil on DB error, got node")
	}
}

func TestGetNode_DBError_ReturnsFalse(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &Store{db: db}

	_, _, ok := store.GetNode("nonexistent_node")
	if ok {
		t.Fatal("expected false on DB error, got true")
	}
}
