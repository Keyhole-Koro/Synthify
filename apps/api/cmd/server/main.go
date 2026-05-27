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

	connect "connectrpc.com/connect"
	apiauth "github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/bootstrap"
	"github.com/synthify/backend/apps/api/internal/config"
	"github.com/synthify/backend/apps/api/internal/handler"
	"github.com/synthify/backend/apps/api/internal/infrastructure/storage"
	"github.com/synthify/backend/apps/api/internal/infrastructure/stripe"
	apiworker "github.com/synthify/backend/apps/api/internal/infrastructure/worker"
	apimiddleware "github.com/synthify/backend/apps/api/internal/middleware"
	"github.com/synthify/backend/apps/api/internal/service"
	appv1connect "github.com/synthify/backend/internal/gen/synthify/app/v1/appv1connect"
	"github.com/synthify/backend/internal/platform/httpmiddleware"
	"github.com/synthify/backend/internal/platform/observability"
)

func main() {
	ctx := context.Background()
	cfg := config.LoadAPI()

	appLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// New Relic is optional: InitNewRelic returns (nil, nil) when no license
	// key is configured, so the app runs fine without observability wired.
	nrApp, err := observability.InitNewRelic(observability.Config{
		AppName:    cfg.NewRelic.AppName,
		LicenseKey: cfg.NewRelic.LicenseKey,
	}, appLogger)
	if err != nil {
		appLogger.Error("api.newrelic_init_failed", "error", err.Error())
	}

	appCtx := bootstrap.Bootstrap(ctx, cfg.GCSBucket, cfg.GCSUploadURLBase, cfg.FirebaseProjectID, appLogger, nrApp)
	store := appCtx.Store
	notifier := appCtx.Notifier

	// Document pipeline wiring. The dispatcher talks to the worker service
	// over Connect; object metadata + source URLs are derived from the
	// internal GCS upload base.
	dispatcher := apiworker.NewHTTPDispatcher(cfg.WorkerBaseURL, observability.ConnectClientOptions(nrApp)...)
	objectMetadata := storage.NewObjectMetadataFetcher(cfg.InternalGCSUploadBase, cfg.GCSBucket)
	sourceURLBuilder := bootstrap.NewDocumentSourceURLBuilder(cfg.GCSBucket, cfg.InternalGCSUploadBase)

	stripeProvider, err := stripe.NewProvider(stripe.Config{
		SecretKey:        cfg.Stripe.SecretKey,
		WebhookSecret:    cfg.Stripe.WebhookSecret,
		ProPriceID:       cfg.Stripe.ProPriceID,
		ProPriceIDJPY:    cfg.Stripe.ProPriceIDJPY,
		ProPriceIDUSD:    cfg.Stripe.ProPriceIDUSD,
		DefaultCurrency:  cfg.Stripe.DefaultCurrency,
		SuccessURL:       cfg.Billing.SuccessURL,
		CancelURL:        cfg.Billing.CancelURL,
		PortalReturnURL:  cfg.Billing.PortalReturnURL,
		APIBase:          cfg.Stripe.APIBase,
		APIVersion:       cfg.Stripe.APIVersion,
		MeterInputEvent:  cfg.Stripe.MeterInputEvent,
		MeterOutputEvent: cfg.Stripe.MeterOutputEvent,
	})
	if err != nil {
		log.Fatalf("stripe provider init: %v", err)
	}
	billingSvc := service.NewBillingService(store, store, stripeProvider, appLogger)

	documentSvc := service.NewDocumentService(
		store, store, store, store, store, store, sourceURLBuilder, objectMetadata, dispatcher, notifier, appLogger,
	)
	itemSvc := service.NewItemService(store, store, store, appLogger)
	workspaceSvc := service.NewWorkspaceService(store, store, appLogger)
	userSvc := service.NewUserService(store, store, billingSvc, appLogger)
	treeSvc := service.NewTreeService(store, store, appLogger)
	authenticator, err := apiauth.NewFirebaseAuthenticator(apiauth.FirebaseAuthenticatorConfig{
		ProjectID:        cfg.FirebaseProjectID,
		ServiceToken:     cfg.Auth.ServiceToken,
		AdminEmailsCSV:   cfg.Auth.AdminEmailsCSV,
		AllowedEmailsCSV: cfg.Auth.AllowedEmailsCSV,
	})
	if err != nil {
		log.Fatalf("authenticator init: %v", err)
	}

	documentHandler := handler.NewDocumentHandler(documentSvc, store)
	treeHandler := handler.NewTreeHandler(treeSvc)
	itemHandler := handler.NewItemHandler(itemSvc)
	workspaceHandler := handler.NewWorkspaceHandler(workspaceSvc)
	userHandler := handler.NewUserHandler(userSvc)
	jobHandler := handler.NewJobHandler(store, store, store, store, store, store, appLogger)
	billingHandler := handler.NewBillingHandler(billingSvc)

	mux := http.NewServeMux()
	connectOptions := observability.ConnectHandlerOptions(nrApp)
	// 内部エラー（DB エラー等）の本文をクライアントに晒さず、サーバ側ログに原因を残す。
	connectOptions = append(connectOptions, connect.WithInterceptors(handler.MaskInternalErrors(appLogger)))
	mux.Handle(appv1connect.NewDocumentServiceHandler(documentHandler, connectOptions...))
	mux.Handle(appv1connect.NewTreeServiceHandler(treeHandler, connectOptions...))
	mux.Handle(appv1connect.NewItemServiceHandler(itemHandler, connectOptions...))
	mux.Handle(appv1connect.NewWorkspaceServiceHandler(workspaceHandler, connectOptions...))
	mux.Handle(appv1connect.NewUserServiceHandler(userHandler, connectOptions...))
	mux.Handle(appv1connect.NewJobServiceHandler(jobHandler, connectOptions...))
	mux.Handle(appv1connect.NewBillingServiceHandler(billingHandler, connectOptions...))

	// Stripe sends raw, signed webhook bodies — this is a plain HTTP
	// endpoint, not Connect. The auth middleware exempts /stripe/webhook.
	mux.HandleFunc("/stripe/webhook", handler.NewBillingWebhookHTTPHandler(billingSvc, appLogger))

	mux.HandleFunc("GET /health", healthHandler(store, cfg.ReadinessKey))

	// Outermost first: recover → log → security headers → CORS → auth → routes.
	// CORS must be outside of Auth to handle preflight (OPTIONS) requests
	// without authentication.
	var h http.Handler = mux
	h = apimiddleware.WithAuth(authenticator, appLogger, h)
	h = apimiddleware.CORS(cfg.CORSAllowedOrigins, h)
	h = apimiddleware.SecurityHeaders(h)
	h = httpmiddleware.Logger(appLogger, h)
	h = httpmiddleware.Recover(appLogger, h)

	addr := fmt.Sprintf(":%s", cfg.Port)
	appLogger.Info("api.started", "addr", addr, "env", cfg.Env)
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
