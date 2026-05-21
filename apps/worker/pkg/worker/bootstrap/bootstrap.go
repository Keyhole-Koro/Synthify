package bootstrap

import (
	"context"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/internal/platform/applog"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/postgres"
)

type AppContext struct {
	Store    Store
	Notifier jobstatus.Notifier
}

func Bootstrap(ctx context.Context, firebaseProjectID string, logger applog.Logger, nrApp *newrelic.Application) *AppContext {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	store := InitStore(ctx, logger, nrApp)
	notifier := jobstatus.NewNotifier(ctx, firebaseProjectID, logger)
	return &AppContext{
		Store:    store,
		Notifier: notifier,
	}
}

type Store interface {
	repository.AccountRepository
	repository.WorkspaceRepository
	repository.DocumentRepository
	repository.TreeRepository
	repository.ItemRepository
	repository.UsageRepository
	repository.CheckpointRepository
	repository.JobLogWriter
	repository.DynamicToolRepository
}

func InitStore(ctx context.Context, logger applog.Logger, nrApp *newrelic.Application) Store {
	if logger == nil {
		logger = applog.NoopLogger{}
	}
	if dsn := config.LoadStore().DatabaseDSN; dsn != "" {
		var lastErr error
		for attempt := 1; attempt <= 10; attempt++ {
			// Worker doesn't need upload URL issuer.
			store, err := postgres.NewStore(ctx, dsn, nil, logger, nrApp)
			if err == nil {
				logger.Info(ctx, "worker.store_initialized", map[string]any{"type": "postgres"})
				return store
			}
			lastErr = err
			logger.Warn(ctx, "worker.store_init_retry", err, map[string]any{"attempt": attempt})
			time.Sleep(2 * time.Second)
		}
		logger.Error(ctx, "worker.store_init_failed", lastErr, nil)
		panic(lastErr)
	}
	logger.Info(ctx, "worker.store_initialized", map[string]any{"type": "mock"})
	return mock.NewStore()
}
