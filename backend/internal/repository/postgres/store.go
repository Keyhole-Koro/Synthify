package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db *sql.DB
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
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func newID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

type scanner interface {
	Scan(dest ...any) error
}
