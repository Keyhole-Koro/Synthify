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

type Store struct {
	db              *sql.DB
	queries         *sqlcgen.Queries
	uploadURLIssuer repository.DocumentUploadURLIssuer
	logger          *slog.Logger
	inTx            bool
}

func NewStore(ctx context.Context, dsn string, uploadURLIssuer repository.DocumentUploadURLIssuer, logger *slog.Logger, nrApp ...*newrelic.Application) (*Store, error) {
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
	return &Store{
		db:              db,
		queries:         sqlcgen.New(db),
		uploadURLIssuer: uploadURLIssuer,
		logger:          logger,
	}, nil
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
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

// WithTx は callback を単一 SQL transaction の中で実行する。
// callback には tx スコープに束縛された Repositories (Store のクローン) が
// 渡される。fn が nil を返せば commit、err を返せば rollback。
//
// 入れ子の WithTx はサポートしない (inTx=true で呼ぶと panic)。
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

func newID() string {
	return util.NewULID()
}

func nowTime() time.Time {
	return util.NowUTC()
}
