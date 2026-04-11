package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

type Store struct {
	db      *sql.DB
	queries *sqlcgen.Queries
}

func NewStore(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db, queries: sqlcgen.New(db)}, nil
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

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func now() string {
	return nowTime().Format(time.RFC3339)
}

func nowTime() time.Time {
	return time.Now().UTC()
}

type scanner interface {
	Scan(dest ...any) error
}
