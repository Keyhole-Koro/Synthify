package postgres

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/oklog/ulid/v2"
	"github.com/synthify/backend/packages/shared/applog"
	"github.com/synthify/backend/packages/shared/repository"
	"github.com/synthify/backend/packages/shared/repository/postgres/sqlcgen"
)

type Store struct {
	db               *sql.DB
	queries          *sqlcgen.Queries
	uploadURLBuilder repository.DocumentUploadURLBuilder
	logger           applog.Logger
}

func NewStore(ctx context.Context, dsn string, uploadURLBuilder repository.DocumentUploadURLBuilder, logger applog.Logger) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	return &Store{
		db:               db,
		queries:          sqlcgen.New(db),
		uploadURLBuilder: uploadURLBuilder,
		logger:           logger,
	}, nil
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
