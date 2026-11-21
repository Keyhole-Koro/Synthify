package database

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestOrDefault(t *testing.T) {
	if got := orDefault(4, 8); got != 4 {
		t.Fatalf("orDefault(4, 8) = %d", got)
	}
	if got := orDefault(0, 8); got != 8 {
		t.Fatalf("orDefault(0, 8) = %d", got)
	}
	if got := orDefault(-1, 8); got != 8 {
		t.Fatalf("orDefault(-1, 8) = %d", got)
	}
}

func TestOpenDBRejectsInvalidDSN(t *testing.T) {
	if db, err := OpenDB("://invalid", PoolConfig{}, nil); err == nil || db != nil {
		t.Fatalf("OpenDB() = db %v, error %v; want parse error", db, err)
	}
}

func TestCheckReadiness(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	if err := CheckReadiness(context.Background(), db); err != nil {
		t.Fatalf("CheckReadiness() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("mock expectations: %v", err)
	}
}
