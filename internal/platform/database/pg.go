package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// Connection pool defaults tuned for CockroachDB Serverless, which bills a
// Request Unit for every new connection's setup. Re-using connections instead
// of churning short-lived ones is the main lever for keeping RU usage low, so
// idle connections are kept around (MaxIdleConns == MaxOpenConns) rather than
// the database/sql default of 2. Lifetime/idle timeouts still recycle stale
// connections so the pool recovers if the server drops them.
const (
	DefaultMaxOpenConns    = 8
	DefaultMaxIdleConns    = 8
	defaultConnMaxIdleTime = 5 * time.Minute
	defaultConnMaxLifetime = 30 * time.Minute
)

// PoolConfig is the per-app connection-pool sizing passed to OpenDB. Each app
// resolves these from its own config (e.g. DB_MAX_OPEN_CONNS / DB_MAX_IDLE_CONNS)
// rather than this shared platform layer reading the environment directly. A
// zero or negative field falls back to the package default, so callers can pass
// a partially-populated config without special-casing unset values.
type PoolConfig struct {
	MaxOpenConns int
	MaxIdleConns int
}

// OpenDB opens a PostgreSQL connection pool with optional New Relic tracing.
// Pool sizing comes from pool; unset (<= 0) fields use the package defaults.
func OpenDB(dsn string, pool PoolConfig, nrApp *newrelic.Application) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if nrApp != nil {
		cfg.Tracer = nrpgx5.NewTracer()
	}
	db := stdlib.OpenDB(*cfg)

	db.SetMaxOpenConns(orDefault(pool.MaxOpenConns, DefaultMaxOpenConns))
	db.SetMaxIdleConns(orDefault(pool.MaxIdleConns, DefaultMaxIdleConns))
	db.SetConnMaxIdleTime(defaultConnMaxIdleTime)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)

	return db, nil
}

// orDefault returns v when it is positive, otherwise def.
func orDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// CheckReadiness performs a simple 'SELECT 1' query to verify the database connection.
func CheckReadiness(ctx context.Context, db *sql.DB) error {
	var one int
	return db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}
