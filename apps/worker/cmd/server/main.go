package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/synthify/backend/apps/worker/pkg/worker"
	"github.com/synthify/backend/apps/worker/pkg/worker/bootstrap"
	"github.com/synthify/backend/apps/worker/pkg/worker/config"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/metering"
	"github.com/synthify/backend/apps/worker/pkg/worker/repository/postgres"
	storage "github.com/synthify/backend/apps/worker/pkg/worker/storage"
	workerv1connect "github.com/synthify/backend/internal/gen/synthify/worker/v1/workerv1connect"
	"github.com/synthify/backend/internal/platform/httpmiddleware"
	joblog "github.com/synthify/backend/internal/platform/job/log"
	"github.com/synthify/backend/internal/platform/observability"
)

func main() {
	ctx := context.Background()
	cfg := config.LoadWorker()

	fs := storage.NewFileSystem(cfg.GCSFuseMountPath)
	appLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	nrApp, err := observability.InitNewRelic(observability.Config{
		AppName:    cfg.NewRelic.AppName,
		LicenseKey: cfg.NewRelic.LicenseKey,
	}, appLogger)
	if err != nil {
		appLogger.Error("worker.newrelic_init_failed", "error", err.Error())
	}

	appCtx := bootstrap.Bootstrap(ctx, cfg.FirebaseProjectID, appLogger, nrApp)
	store := appCtx.Store
	notifier := appCtx.Notifier
	jobLogger := postgres.NewDBLogger(store)

	adkModel, embedder := llm.Init(ctx, config.LoadLLM(), fs, appLogger)
	// One reporter feeds both metering paths: the llm.Client wrapper (embedding
	// + custom-client calls) and the agent's after-model callback (ADK
	// generation calls, which never cross the llm.Client surface).
	reporter := metering.NewConnectReporter(cfg.APIBaseURL, cfg.InternalServiceToken, observability.ConnectClientOptions(nrApp)...)
	llmClient := metering.NewLLMClient(embedder, reporter, appLogger)

	workerService, err := worker.NewWorkerWithNotifier(store, store, notifier, adkModel, embedder, llmClient, reporter, fs, appLogger, nrApp)
	if err != nil {
		log.Fatal(err)
	}
	planner := worker.NewPlanner(store, adkModel, appLogger)
	evaluator := worker.NewJobEvaluator(store, embedder, appLogger)

	mux := http.NewServeMux()
	handlerOpts := append(
		observability.ConnectHandlerOptions(nrApp),
		observability.MaskInternalErrorsHandlerOptions(appLogger)...,
	)
	mux.Handle(workerv1connect.NewWorkerServiceHandler(
		worker.NewConnectHandler(workerService, store, planner, evaluator, appLogger),
		handlerOpts...,
	))
	// Cloud Tasks delivers job dispatches here as plain JSON POSTs. The
	// Connect RPCs above still exist for direct API->worker calls in local
	// runs that bypass Cloud Tasks. Authentication is enforced upstream by
	// Cloud Run (allow_unauthenticated=false + run.invoker on the Cloud
	// Tasks SA).
	mux.Handle("POST /internal/dispatch-job",
		worker.NewInternalDispatchHandler(workerService, planner, appLogger))
	mux.HandleFunc("GET /health", healthHandler(store, cfg.ReadinessKey))

	addr := fmt.Sprintf(":%s", cfg.Port)
	appLogger.Info("worker.started", "addr", addr)
	h := httpmiddleware.Recover(appLogger, httpmiddleware.Logger(appLogger, withJobLogger(jobLogger, mux)))
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
