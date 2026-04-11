package postgres

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestCreateWorkspace_CommitsTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	store := &Store{db: db}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO workspaces (workspace_id, name, owner_id, plan, storage_used_bytes, storage_quota_bytes, max_file_size_bytes, max_uploads_per_day, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`)).
		WithArgs(
			sqlmock.AnyArg(),
			"Test Workspace",
			"user_demo",
			"free",
			int64(0),
			int64(1<<30),
			int64(50<<20),
			int64(10),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO workspace_members (workspace_id, user_id, email, role, is_dev, invited_at, invited_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`)).
		WithArgs(
			sqlmock.AnyArg(),
			"user_demo",
			"demo@synthify.dev",
			"owner",
			true,
			sqlmock.AnyArg(),
			"user_demo",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	workspace := store.CreateWorkspace("Test Workspace")
	if workspace == nil {
		t.Fatal("CreateWorkspace returned nil")
	}
	if workspace.Name != "Test Workspace" {
		t.Fatalf("workspace.Name = %q, want %q", workspace.Name, "Test Workspace")
	}
	if workspace.OwnerID != "user_demo" {
		t.Fatalf("workspace.OwnerID = %q, want user_demo", workspace.OwnerID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
