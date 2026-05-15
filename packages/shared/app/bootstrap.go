package app

import (
	"context"
	"time"

	"github.com/synthify/backend/packages/shared/applog"
	"github.com/synthify/backend/packages/shared/config"
	jobstatus "github.com/synthify/backend/packages/shared/job/status"
	"github.com/synthify/backend/packages/shared/repository"
	"github.com/synthify/backend/packages/shared/repository/mock"
	"github.com/synthify/backend/packages/shared/repository/postgres"
	"github.com/synthify/backend/packages/shared/storage"
)

type AppContext struct {
	Store    Store
	Notifier jobstatus.Notifier
}

func Bootstrap(ctx context.Context, gcsURLBase, firebaseProjectID string, logger applog.Logger) *AppContext {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	store := InitStore(ctx, NewDocumentUploadURLBuilder(gcsURLBase), logger)
	notifier := jobstatus.NewNotifier(ctx, firebaseProjectID, logger)
	return &AppContext{
		Store:    store,
		Notifier: notifier,
	}
}

// Store aggregates every persistence-facing capability via embedded
// domain-specific repository interfaces. Callers should prefer depending
// on a narrower interface from packages/shared/repository when possible.
type Store interface {
	repository.AccountRepository
	repository.WorkspaceRepository
	repository.DocumentRepository
	repository.TreeRepository
	repository.ItemRepository
	repository.UsageRepository
	repository.CheckpointRepository
	repository.JobLogWriter
}

func NewDocumentUploadURLBuilder(base string) repository.DocumentUploadURLBuilder {
	return func(workspaceID, documentID string) string {
		return storage.BuildDocumentUploadURL(base, workspaceID, documentID)
	}
}

func NewDocumentSourceURLBuilder(base string) repository.DocumentSourceURLBuilder {
	return func(workspaceID, documentID string) string {
		return storage.BuildDocumentSourceURL(base, workspaceID, documentID)
	}
}

func InitStore(ctx context.Context, uploadURLBuilder repository.DocumentUploadURLBuilder, logger applog.Logger) Store {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	if dsn := config.LoadStore().DatabaseURL; dsn != "" {
		var lastErr error
		for attempt := 1; attempt <= 10; attempt++ {
			store, err := postgres.NewStore(ctx, dsn, uploadURLBuilder, logger)
			if err == nil {
				logger.Info(ctx, "app.store_initialized", map[string]any{"type": "postgres"})
				return store
			}
			lastErr = err
			logger.Warn(ctx, "app.store_init_retry", err, map[string]any{"attempt": attempt})
			time.Sleep(2 * time.Second)
		}
		logger.Error(ctx, "app.store_init_failed", lastErr, nil)
		panic(lastErr)
	}
	logger.Info(ctx, "app.store_initialized", map[string]any{"type": "mock"})
	return mock.NewStore()
}
