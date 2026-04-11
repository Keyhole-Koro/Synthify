package postgres

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestCreateNode_WithParentNode_CreatesHierarchicalEdge(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &Store{db: db}

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO nodes`).
		WithArgs(
			sqlmock.AnyArg(),
			"doc_1",
			"New Node",
			2,
			"concept",
			"",
			"Generated during test",
			"",
			"user_1",
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO edges`).
		WithArgs(
			sqlmock.AnyArg(),
			"doc_1",
			"nd_parent",
			sqlmock.AnyArg(),
			"hierarchical",
			"",
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE documents`).
		WithArgs("doc_1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	node := store.CreateNode("doc_1", "New Node", "concept", "Generated during test", "nd_parent", 2, "user_1")
	if node == nil {
		t.Fatal("CreateNode returned nil")
	}
	if node.DocumentID != "doc_1" {
		t.Fatalf("node.DocumentID = %q, want doc_1", node.DocumentID)
	}
	if node.Level != 2 {
		t.Fatalf("node.Level = %d, want 2", node.Level)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
