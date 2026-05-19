package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/metering"
	"github.com/synthify/backend/packages/shared/app"
	"github.com/synthify/backend/packages/shared/applog"
	"github.com/synthify/backend/packages/shared/config"
	treev1connect "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1/treev1connect"
	"github.com/synthify/backend/packages/shared/job/log"
	"github.com/synthify/backend/packages/shared/middleware"
	"github.com/synthify/backend/packages/shared/observability"
	"github.com/synthify/backend/packages/shared/repository/postgres"
	"github.com/synthify/backend/packages/shared/storage"
)

func main() {
	ctx := context.Background()
	cfg := config.LoadWorker()

	fs := storage.NewFileSystem(cfg.GCSFuseMountPath)
	slogLogger := applog.NewJSONSlogLogger(os.Stdout)
	appLogger := applog.WrapSlogLogger(slogLogger)

	nrApp, err := observability.InitNewRelic(cfg.NewRelic, slogLogger)
	if err != nil {
		appLogger.Error(ctx, "worker.newrelic_init_failed", err, nil)
	}

	appCtx := app.Bootstrap(ctx, cfg.GCSBucket, cfg.GCSUploadURLBase, cfg.FirebaseProjectID, appLogger, nrApp)
	store := appCtx.Store
	notifier := appCtx.Notifier
	jobLogger := postgres.NewDBLogger(store)

	adkModel, embedder := llm.Init(ctx, config.LoadLLM(), fs, appLogger)
	llmClient := metering.NewWrappedClient(embedder, cfg, appLogger, observability.ConnectClientOptions(nrApp)...)

	workerService, err := worker.NewWorkerWithNotifier(store, store, notifier, adkModel, embedder, llmClient, fs, appLogger)
	if err != nil {
		log.Fatal(err)
	}
	planner := worker.NewPlanner(store, adkModel, appLogger)
	evaluator := worker.NewJobEvaluator(store, embedder, appLogger)

	mux := http.NewServeMux()
	mux.Handle(treev1connect.NewWorkerServiceHandler(
		worker.NewConnectHandler(workerService, store, planner, evaluator, appLogger),
		observability.ConnectHandlerOptions(nrApp)...,
	))
	mux.HandleFunc("GET /health", healthHandler(store, cfg.ReadinessKey))

	addr := fmt.Sprintf(":%s", cfg.Port)
	appLogger.Info(ctx, "worker.started", map[string]any{"addr": addr})
	h := middleware.Recover(appLogger, middleware.Logger(appLogger, withJobLogger(jobLogger, mux)))
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}

type readinessChecker interface {
	CheckReadiness(context.Context) error
}

func healthHandler(store any, readinessKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ready") != "1" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"status":"ok"}`)
			return
		}

		// Readiness is used by deploy smoke tests and exposes dependency
		// status, so it is protected separately from the public liveness check.
		if !readinessAuthorized(r, readinessKey) {
			http.Error(w, `{"status":"error","error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		checker, ok := store.(readinessChecker)
		if !ok {
			http.Error(w, `{"status":"error","dependency":"store"}`, http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := checker.CheckReadiness(ctx); err != nil {
			http.Error(w, `{"status":"error","dependency":"store"}`, http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok","ready":true}`)
	}
}

func readinessAuthorized(r *http.Request, expected string) bool {
	expected = strings.TrimSpace(expected)
	got := strings.TrimSpace(r.Header.Get("X-Synthify-Readiness-Key"))
	if expected == "" || got == "" {
		return false
	}
	expectedHash := sha256.Sum256([]byte(expected))
	gotHash := sha256.Sum256([]byte(got))
	return subtle.ConstantTimeCompare(gotHash[:], expectedHash[:]) == 1
}

func withJobLogger(l joblog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := joblog.WithLogger(r.Context(), l)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
