package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/synthify/backend/apps/api/internal/bootstrap"
	"github.com/synthify/backend/apps/api/internal/config"
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

	logger.Info("api.started", "addr", app.Address, "env", cfg.Env)
	// Same reasoning as the worker: http.ListenAndServe leaves every timeout at
	// zero, so a client stalling mid-header pins a goroutine. Write/ReadTimeout
	// are left unset because handlers here fan out to Firestore, GCS and Stripe
	// and a blanket response deadline would cut those off; document bodies never
	// pass through this server, they go straight to GCS via signed URLs.
	srv := &http.Server{
		Addr:              app.Address,
		Handler:           app.Handler,
		ReadHeaderTimeout: 20 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
