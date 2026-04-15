package postgres

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestCreateWorkspace_DBError_ReturnsNil(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &Store{db: db}

	// No expectations set → any query returns an error.
	ws := store.CreateWorkspace("acc_1", "Test Workspace")
	if ws != nil {
		t.Fatal("expected nil on DB error, got workspace")
	}
}

func TestGetWorkspace_DBError_ReturnsFalse(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &Store{db: db}

	_, ok := store.GetWorkspace("nonexistent_id")
	if ok {
		t.Fatal("expected false on DB error, got true")
	}
}

func TestIsWorkspaceAccessible_DBError_ReturnsFalse(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &Store{db: db}

	if store.IsWorkspaceAccessible("ws_1", "user_1") {
		t.Fatal("expected false on DB error, got true")
	}
}
