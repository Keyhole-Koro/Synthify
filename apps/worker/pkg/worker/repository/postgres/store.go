package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/postgres/sqlcgen"
	"github.com/synthify/backend/internal/platform/applog"
	"github.com/synthify/backend/internal/platform/database"
	"github.com/synthify/backend/internal/platform/util"
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

	db, err := database.OpenDB(dsn, app)
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

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CheckReadiness(ctx context.Context) error {
	return database.CheckReadiness(ctx, s.db)
}

func (s *Store) q() *sqlcgen.Queries {
	if s.queries == nil {
		s.queries = sqlcgen.New(s.db)
	}
	return s.queries
}

func newID() string {
	return util.NewULID()
}

func nowTime() time.Time {
	return util.NowUTC()
}
