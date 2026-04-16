package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Keyhole-Koro/SynthifyShared/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/oklog/ulid/v2"
	"github.com/synthify/backend/internal/repository/postgres/sqlcgen"
)

type Store struct {
	db                 *sql.DB
	queries            *sqlcgen.Queries
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
	store := &Store{db: db, queries: sqlcgen.New(db), uploadURLGenerator: uploadURLGenerator}
	if err := store.ensureSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
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

func (s *Store) ensureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS document_chunks (
			chunk_id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL REFERENCES documents(document_id) ON DELETE CASCADE,
			heading TEXT NOT NULL DEFAULT '',
			text TEXT NOT NULL,
			source_page INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_document_chunks_document_id ON document_chunks(document_id)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}
