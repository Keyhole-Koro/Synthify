package postgres

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/oklog/ulid/v2"
	"github.com/synthify/backend/internal/repository"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

type Store struct {
	db                  *sql.DB
	queries             *sqlcgen.Queries
	uploadURLGenerator repository.UploadURLGenerator
}

func NewStore(ctx context.Context, dsn string, uploadURLGenerator repository.UploadURLGenerator) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, queries: sqlcgen.New(db), uploadURLGenerator: uploadURLGenerator}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) q() *sqlcgen.Queries {
	if s.queries == nil {
		s.queries = sqlcgen.New(s.db)
	}
	return s.queries
}

func newID() string {
	return ulid.Make().String()
}

func nowTime() time.Time {
	return time.Now().UTC()
}

type scanner interface {
	Scan(dest ...any) error
}
