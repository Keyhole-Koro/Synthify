package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/synthify/backend/apps/api/internal/bootstrap"
	"github.com/synthify/backend/apps/api/internal/config"
	"github.com/synthify/backend/internal/platform/httpserver"
	"github.com/synthify/backend/internal/platform/observability"
)

func main() {
	ctx := context.Background()
	cfg := config.LoadAPI()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	nrApp, err := observability.InitNewRelic(observability.Config{
		AppName:    cfg.NewRelic.AppName,
		LicenseKey: cfg.NewRelic.LicenseKey,
	}, logger)
	if err != nil {
		logger.Error("api.newrelic_init_failed", "error", err.Error())
	}

	app, err := bootstrap.NewApplication(ctx, cfg, logger, nrApp)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			logger.Error("api.close_failed", "error", err.Error())
		}
	}()

	logger.Info("api.started", "addr", app.Address, "env", cfg.Env, "intake_closed", cfg.IntakeClosed)
	if err := httpserver.New(app.Address, app.Handler).ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
