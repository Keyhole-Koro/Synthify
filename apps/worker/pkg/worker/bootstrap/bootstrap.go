package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/mock"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/postgres"
	jobstatus "github.com/synthify/backend/internal/platform/job/status"
)

type AppContext struct {
	Store    Store
	Notifier jobstatus.Notifier
}

func Bootstrap(ctx context.Context, firebaseProjectID string, logger *slog.Logger, nrApp *newrelic.Application) *AppContext {
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

func InitStore(ctx context.Context, logger *slog.Logger, nrApp *newrelic.Application) Store {
	if dsn := config.LoadStore().DatabaseDSN; dsn != "" {
		var lastErr error
		for attempt := 1; attempt <= 10; attempt++ {
			// Worker doesn't need upload URL issuer.
			store, err := postgres.NewStore(ctx, dsn, nil, logger, nrApp)
			if err == nil {
				logger.Info("worker.store_initialized", "type", "postgres")
				return store
			}
			lastErr = err
			logger.Warn("worker.store_init_retry", "error", err.Error(), "attempt", attempt)
			time.Sleep(2 * time.Second)
		}
		logger.Error("worker.store_init_failed", "error", lastErr.Error())
		panic(lastErr)
	}
	logger.Info("worker.store_initialized", "type", "mock")
	return mock.NewStore()
}
