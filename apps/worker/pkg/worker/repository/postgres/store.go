// Package postgres provides PostgreSQL implementations of the worker repository interfaces.
package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/postgres/sqlcgen"
	"github.com/synthify/backend/internal/platform/database"
	"github.com/synthify/backend/internal/platform/util"
)

// Compile-time check: *Store satisfies both the unit-of-work boundary
// (Transactor) and the bundle of all repositories (Repositories).
var (
	_ repository.Transactor   = (*Store)(nil)
	_ repository.Repositories = (*Store)(nil)
)

// Store provides PostgreSQL data access implementations across all domain entities.
type Store struct {
	db              *sql.DB
	queries         *sqlcgen.Queries
	uploadURLIssuer repository.DocumentUploadURLIssuer
	logger          *slog.Logger
	inTx            bool
}

// NewStore initializes a new Store with a configured database connection pool and query engine.
func NewStore(ctx context.Context, dsn string, pool database.PoolConfig, uploadURLIssuer repository.DocumentUploadURLIssuer, logger *slog.Logger, nrApp ...*newrelic.Application) (*Store, error) {
	var app *newrelic.Application
	if len(nrApp) > 0 {
		app = nrApp[0]
	}

	db, err := database.OpenDB(dsn, pool, app)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{
		db:              db,
		queries:         sqlcgen.New(db),
		uploadURLIssuer: uploadURLIssuer,
		logger:          logger,
	}, nil
}

// Close closes the underlying SQL database connection pool.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// CheckReadiness verifies database connectivity by executing a ping against the connection pool.
func (s *Store) CheckReadiness(ctx context.Context) error {
	return database.CheckReadiness(ctx, s.db)
}

// q returns the active sqlc Queries instance, falling back to initializing with s.db if nil.
func (s *Store) q() *sqlcgen.Queries {
	if s.queries == nil {
		s.queries = sqlcgen.New(s.db)
	}
	return s.queries
}

// WithTx executes the callback within a single SQL transaction.
// The callback receives a transaction-bound Repositories implementation (a Store clone).
// If fn returns nil, the transaction commits; otherwise it rolls back.
//
// Nested transactions are not supported and will panic if invoked on a transaction-bound Store.
func (s *Store) WithTx(ctx context.Context, fn func(repository.Repositories) error) error {
	if s.inTx {
		panic("postgres.Store.WithTx: nested transactions are not supported")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	txStore := &Store{
		db:              s.db,
		queries:         s.queries.WithTx(tx),
		uploadURLIssuer: s.uploadURLIssuer,
		logger:          s.logger,
		inTx:            true,
	}
	if err := fn(txStore); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// newID generates a new ULID string for database records.
func newID() string {
	return util.NewULID()
}

// nowTime returns the current UTC timestamp.
func nowTime() time.Time {
	return util.NowUTC()
}
