package postgres

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/api/internal/repository"
	"github.com/synthify/backend/apps/api/internal/repository/postgres/sqlcgen"
	"github.com/synthify/backend/internal/platform/database"
	"github.com/synthify/backend/internal/platform/util"
)

// Compile-time check: *Store satisfies both the unit-of-work boundary
// (Transactor) and the bundle of all repositories (Repositories).
var (
	_ repository.Transactor    = (*Store)(nil)
	_ repository.Repositories  = (*Store)(nil)
)

// Store provides the PostgreSQL implementation of repository.Repositories and repository.Transactor.
type Store struct {
	db              *sql.DB
	queries         *sqlcgen.Queries
	uploadURLIssuer repository.DocumentUploadURLIssuer
	logger          *slog.Logger
	inTx            bool
}

// NewStore initializes a PostgreSQL database connection pool, checks connectivity with Ping, and returns a Store.
func NewStore(ctx context.Context, dsn string, pool database.PoolConfig, uploadURLIssuer repository.DocumentUploadURLIssuer, logger *slog.Logger, nrApp ...*newrelic.Application) (*Store, error) {
	store, db, err := newStoreUnpinged(dsn, pool, uploadURLIssuer, logger, nrApp...)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// NewStoreWithoutPing builds the store without verifying connectivity up front.
// database/sql connects lazily, so the pool will establish (and re-establish)
// connections on first use and recover if the database was unreachable at
// startup. Used by bootstrap to keep the API process alive — and its liveness
// probe green — even when the database is temporarily down, instead of crash-
// looping. DB-backed RPCs still fail until the database is reachable again.
func NewStoreWithoutPing(dsn string, pool database.PoolConfig, uploadURLIssuer repository.DocumentUploadURLIssuer, logger *slog.Logger, nrApp ...*newrelic.Application) (*Store, error) {
	store, _, err := newStoreUnpinged(dsn, pool, uploadURLIssuer, logger, nrApp...)
	return store, err
}

// newStoreUnpinged opens the database connection pool with optional New Relic instrumentation.
func newStoreUnpinged(dsn string, pool database.PoolConfig, uploadURLIssuer repository.DocumentUploadURLIssuer, logger *slog.Logger, nrApp ...*newrelic.Application) (*Store, *sql.DB, error) {
	var app *newrelic.Application
	if len(nrApp) > 0 {
		app = nrApp[0]
	}

	db, err := database.OpenDB(dsn, pool, app)
	if err != nil {
		return nil, nil, err
	}
	return &Store{
		db:              db,
		queries:         sqlcgen.New(db),
		uploadURLIssuer: uploadURLIssuer,
		logger:          logger,
	}, db, nil
}

// Close closes the underlying PostgreSQL connection pool.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// CheckReadiness validates that PostgreSQL is reachable and accepting queries.
func (s *Store) CheckReadiness(ctx context.Context) error {
	return database.CheckReadiness(ctx, s.db)
}

// q returns the active sqlc Queries instance, initializing it if nil.
func (s *Store) q() *sqlcgen.Queries {
	if s.queries == nil {
		s.queries = sqlcgen.New(s.db)
	}
	return s.queries
}

// WithTx executes the callback within a single SQL transaction.
// A tx-scoped Repositories instance is passed to the callback.
// If fn returns nil, the transaction commits; if it returns an error, it rolls back.
// Nested WithTx calls are not supported and will panic.
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

// newID generates a new random monotonic ULID string.
func newID() string {
	return util.NewULID()
}

// nowTime returns current UTC timestamp.
func nowTime() time.Time {
	return util.NowUTC()
}
