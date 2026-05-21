package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/oklog/ulid/v2"
	"github.com/synthify/backend/internal/platform/applog"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/apps/api/internal/repository/postgres/sqlcgen"
)

type Store struct {
	db              *sql.DB
	queries         *sqlcgen.Queries
	uploadURLIssuer repository.DocumentUploadURLIssuer
	logger          applog.Logger
}

func NewStore(ctx context.Context, dsn string, uploadURLIssuer repository.DocumentUploadURLIssuer, logger applog.Logger, nrApp ...*newrelic.Application) (*Store, error) {
	var app *newrelic.Application
	if len(nrApp) > 0 {
		app = nrApp[0]
	}

	db, err := openDB(dsn, app)
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
		db:              db,
		queries:         sqlcgen.New(db),
		uploadURLIssuer: uploadURLIssuer,
		logger:          logger,
	}, nil
}

func openDB(dsn string, nrApp *newrelic.Application) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if nrApp != nil {
		cfg.Tracer = nrpgx5.NewTracer()
	}
	return stdlib.OpenDB(*cfg), nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CheckReadiness(ctx context.Context) error {
	var one int
	return s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
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
