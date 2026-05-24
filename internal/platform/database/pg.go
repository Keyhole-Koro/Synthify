package database

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/newrelic/go-agent/v3/integrations/nrpgx5"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// OpenDB opens a PostgreSQL connection with optional New Relic tracing.
func OpenDB(dsn string, nrApp *newrelic.Application) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if nrApp != nil {
		cfg.Tracer = nrpgx5.NewTracer()
	}
	return stdlib.OpenDB(*cfg), nil
}

// CheckReadiness performs a simple 'SELECT 1' query to verify the database connection.
func CheckReadiness(ctx context.Context, db *sql.DB) error {
	var one int
	return db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}
