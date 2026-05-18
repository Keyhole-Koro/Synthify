package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/synthify/backend/apps/api/internal/handler"
	"github.com/synthify/backend/apps/api/internal/infrastructure/storage"
	"github.com/synthify/backend/apps/api/internal/infrastructure/stripe"
	apiworker "github.com/synthify/backend/apps/api/internal/infrastructure/worker"
	"github.com/synthify/backend/apps/api/internal/service"
	"github.com/synthify/backend/packages/shared/app"
	"github.com/synthify/backend/packages/shared/applog"
	"github.com/synthify/backend/packages/shared/config"
	treev1connect "github.com/synthify/backend/packages/shared/gen/synthify/tree/v1/treev1connect"
	"github.com/synthify/backend/packages/shared/middleware"
	"github.com/synthify/backend/packages/shared/observability"
)

func main() {
	ctx := context.Background()
	cfg := config.LoadAPI()

	slogLogger := applog.NewJSONSlogLogger(os.Stdout)
	appLogger := applog.WrapSlogLogger(slogLogger)

	// New Relic is optional: InitNewRelic returns (nil, nil) when no license
	// key is configured, so the app runs fine without observability wired.
	nrApp, err := observability.InitNewRelic(cfg.NewRelic, slogLogger)
	if err != nil {
		appLogger.Error(ctx, "api.newrelic_init_failed", err, nil)
	}

	appCtx := app.Bootstrap(ctx, cfg.GCSBucket, cfg.GCSUploadURLBase, cfg.FirebaseProjectID, appLogger, nrApp)
	store := appCtx.Store
	notifier := appCtx.Notifier

	// Document pipeline wiring. The dispatcher talks to the worker service
	// over Connect; object metadata + source URLs are derived from the
	// internal GCS upload base.
	dispatcher := apiworker.NewHTTPDispatcher(cfg.WorkerBaseURL, observability.ConnectClientOptions(nrApp)...)
	objectMetadata := storage.NewObjectMetadataFetcher(cfg.InternalGCSUploadBase, cfg.GCSBucket)
	sourceURLBuilder := app.NewDocumentSourceURLBuilder(cfg.GCSBucket, cfg.InternalGCSUploadBase)
	uploadURLIssuer := app.NewDocumentUploadURLIssuer(cfg.GCSBucket, cfg.GCSUploadURLBase)
	documentSvc := service.NewDocumentService(
		store, store, sourceURLBuilder, objectMetadata, dispatcher, notifier, appLogger,
	)
	itemSvc := service.NewItemService(store, store, appLogger)
	workspaceSvc := service.NewWorkspaceService(store, store, appLogger)

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

	documentHandler := handler.NewDocumentHandler(documentSvc, store, store, uploadURLIssuer)
	treeHandler := handler.NewTreeHandler(store, store, store)
	itemHandler := handler.NewItemHandler(itemSvc, store, store)
	workspaceHandler := handler.NewWorkspaceHandler(workspaceSvc, billingSvc, store)
	jobHandler := handler.NewJobHandler(store, store, store, appLogger)
	billingHandler := handler.NewBillingHandler(billingSvc)

	mux := http.NewServeMux()
	connectOptions := observability.ConnectHandlerOptions(nrApp)
	mux.Handle(treev1connect.NewDocumentServiceHandler(documentHandler, connectOptions...))
	mux.Handle(treev1connect.NewTreeServiceHandler(treeHandler, connectOptions...))
	mux.Handle(treev1connect.NewItemServiceHandler(itemHandler, connectOptions...))
	mux.Handle(treev1connect.NewWorkspaceServiceHandler(workspaceHandler, connectOptions...))
	mux.Handle(treev1connect.NewJobServiceHandler(jobHandler, connectOptions...))
	mux.Handle(treev1connect.NewBillingServiceHandler(billingHandler, connectOptions...))

	// Stripe sends raw, signed webhook bodies — this is a plain HTTP
	// endpoint, not Connect. The auth middleware exempts /stripe/webhook.
	mux.HandleFunc("/stripe/webhook", handler.NewBillingWebhookHTTPHandler(billingSvc, appLogger))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	// Outermost first: recover → log → CORS → auth → routes.
	// CORS must be outside of Auth to handle preflight (OPTIONS) requests
	// without authentication.
	var h http.Handler = mux
	h = middleware.WithAuth(cfg.FirebaseProjectID, appLogger, h)
	h = middleware.CORS(cfg.CORSAllowedOrigins, h)
	h = middleware.Logger(appLogger, h)
	h = middleware.Recover(appLogger, h)

	addr := fmt.Sprintf(":%s", cfg.Port)
	appLogger.Info(ctx, "api.started", map[string]any{"addr": addr, "env": cfg.Env})
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}
