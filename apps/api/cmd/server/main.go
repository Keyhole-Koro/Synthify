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

	// Document pipeline wiring. In stage/prod the dispatcher pushes jobs
	// through Cloud Tasks (set via WORKER_CLOUDTASKS_QUEUE +
	// WORKER_DISPATCH_URL + WORKER_INVOKER_SA), so the API responds
	// immediately and Cloud Tasks handles cold-start retries on the
	// worker side. Local runs that leave these unset still get the
	// legacy synchronous Connect dispatcher driven by WORKER_BASE_URL.
	var dispatcher service.WorkerDispatcher
	if cfg.WorkerDispatch.CloudTasksQueue != "" {
		ctDispatcher, err := apiworker.NewCloudTasksDispatcher(ctx, apiworker.CloudTasksDispatcherConfig{
			QueuePath:        cfg.WorkerDispatch.CloudTasksQueue,
			DispatchURL:      cfg.WorkerDispatch.DispatchURL,
			InvokerSA:        cfg.WorkerDispatch.InvokerSA,
			OIDCAudience:     cfg.WorkerDispatch.OIDCAudience,
			DispatchDeadline: cfg.WorkerDispatch.DispatchDeadline,
			Logger:           appLogger,
		})
		if err != nil {
			log.Fatalf("cloud tasks dispatcher init: %v", err)
		}
		defer ctDispatcher.Close()
		dispatcher = ctDispatcher
	} else {
		dispatcher = apiworker.NewHTTPDispatcher(cfg.WorkerBaseURL, appLogger, observability.ConnectClientOptions(nrApp)...)
	}
	objectMetadata := storage.NewObjectMetadataFetcher(cfg.InternalGCSUploadBase, cfg.GCSBucket)
	objectStore := storage.NewDocumentObjectStore(cfg.InternalGCSUploadBase, cfg.GCSBucket)
	sourceURLBuilder := bootstrap.NewDocumentSourceURLBuilder(cfg.GCSBucket, cfg.InternalGCSUploadBase)

	stripeProvider, err := stripe.NewProvider(stripe.Config{
		SecretKey:        cfg.Stripe.SecretKey,
		WebhookSecret:    cfg.Stripe.WebhookSecret,
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
	// In production/staging the usage repository must be wired, or RecordUsage
	// silently falls back to a logging stub that bills nothing. RequireUsage
	// makes that a startup error instead of a silent revenue leak; dev/local
	// leave it false so they can run without the usage tables.
	billingSvc, err := service.NewBillingService(service.BillingServiceDeps{
		Accounts:     store,
		Usage:        store,
		Provider:     stripeProvider,
		Logger:       appLogger,
		RequireUsage: requiresBilling(cfg.Env),
	})
	if err != nil {
		log.Fatalf("api.billing_init: %v (ENV=%s)", err, cfg.Env)
	}

	documentSvc := service.NewDocumentService(service.DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: sourceURLBuilder,
		ObjectMetadata:   objectMetadata,
		ObjectStore:      objectStore,
		Dispatcher:       dispatcher,
		Notifier:         notifier,
		Logger:           appLogger,
		NRApp:            nrApp,
	})
	go func() {
		// Let the new API/worker revisions settle, then retry the latest failed
		// document jobs once. The retry job carries retry_count=1, so subsequent
		// API restarts do not loop forever on the same failure.
		time.Sleep(15 * time.Second)
		resumeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		resumed, err := documentSvc.AutoResumeFailedJobs(resumeCtx)
		if err != nil {
			appLogger.Error("job.auto_resume_scan_failed", "error", err.Error())
			return
		}
		if resumed > 0 {
			appLogger.Info("job.auto_resume_completed", "resumed", resumed)
		}
	}()
	itemSvc := service.NewItemService(service.ItemServiceDeps{
		Repo:       store,
		Workspaces: store,
		Logger:     appLogger,
	})
	workspaceSvc := service.NewWorkspaceService(service.WorkspaceServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Logger:     appLogger,
	})
	userSvc := service.NewUserService(service.UserServiceDeps{
		Users:    store,
		Accounts: store,
		Billing:  billingSvc,
		Logger:   appLogger,
	})
	treeSvc := service.NewTreeService(service.TreeServiceDeps{
		Tree:       store,
		Workspaces: store,
		Logger:     appLogger,
	})

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
	jobHandler := handler.NewJobHandler(store, store, store, store, store, appLogger)
	billingHandler := handler.NewBillingHandler(billingSvc)

	mux := http.NewServeMux()
	connectOptions := append(
		observability.ConnectHandlerOptions(nrApp),
		observability.MaskInternalErrorsHandlerOptions(appLogger)...,
	)
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

// requiresBilling reports whether the environment must have a fully wired
// billing pipeline. production/staging fail fast on a missing usage repository
// (which would otherwise bill nothing); dev/local stay lenient.
func requiresBilling(env string) bool {
	switch env {
	case "production", "staging":
		return true
	default:
		return false
	}
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
