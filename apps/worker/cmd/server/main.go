package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/synthify/backend/apps/worker/pkg/worker"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/metering"
	"github.com/synthify/backend/packages/shared/app"
	"github.com/synthify/backend/packages/shared/applog"
	"github.com/synthify/backend/packages/shared/config"
	treev1connect "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1/treev1connect"
	"github.com/synthify/backend/packages/shared/job/log"
	"github.com/synthify/backend/packages/shared/middleware"
	"github.com/synthify/backend/packages/shared/repository/postgres"
	"github.com/synthify/backend/packages/shared/storage"
)

func main() {
	ctx := context.Background()
	cfg := config.LoadWorker()

	fs := storage.NewFileSystem(cfg.GCSFuseMountPath)
	appLogger := applog.NewStdLogger()

	appCtx := app.Bootstrap(ctx, cfg.GCSBucket, cfg.GCSUploadURLBase, cfg.FirebaseProjectID, appLogger, nil)
	store := appCtx.Store
	notifier := appCtx.Notifier
	jobLogger := postgres.NewDBLogger(store)

	adkModel, embedder := llm.Init(ctx, config.LoadLLM(), fs, appLogger)
	llmClient := metering.NewWrappedClient(embedder, cfg, appLogger)

	workerService, err := worker.NewWorkerWithNotifier(store, store, notifier, adkModel, embedder, llmClient, fs, appLogger)
	if err != nil {
		log.Fatal(err)
	}
	planner := worker.NewPlanner(store, adkModel, appLogger)
	evaluator := worker.NewJobEvaluator(store, embedder, appLogger)

	mux := http.NewServeMux()
	mux.Handle(treev1connect.NewWorkerServiceHandler(worker.NewConnectHandler(workerService, store, planner, evaluator, appLogger)))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	appLogger.Info(ctx, "worker.started", map[string]any{"addr": addr})
	h := middleware.Recover(appLogger, middleware.Logger(appLogger, withJobLogger(jobLogger, mux)))
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}

func withJobLogger(l joblog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := joblog.WithLogger(r.Context(), l)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
