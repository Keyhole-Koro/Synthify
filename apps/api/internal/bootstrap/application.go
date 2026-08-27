package bootstrap

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/api/internal/application"
	apiauth "github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/config"
	"github.com/synthify/backend/apps/api/internal/handler"
	"github.com/synthify/backend/apps/api/internal/infrastructure/storage"
	"github.com/synthify/backend/apps/api/internal/infrastructure/stripe"
	apiworker "github.com/synthify/backend/apps/api/internal/infrastructure/worker"
	apimiddleware "github.com/synthify/backend/apps/api/internal/middleware"
	appv1connect "github.com/synthify/backend/internal/gen/synthify/app/v1/appv1connect"
	"github.com/synthify/backend/internal/platform/httpmiddleware"
	"github.com/synthify/backend/internal/platform/observability"
)

// Application is the fully wired API process. bootstrap is the only package
// that selects concrete infrastructure implementations and connects them to
// application services and transport handlers.
type Application struct {
	Handler http.Handler
	Address string
	close   func() error
}

func (a *Application) Close() error {
	if a == nil || a.close == nil {
		return nil
	}
	return a.close()
}

func NewApplication(ctx context.Context, cfg config.API, logger *slog.Logger, nrApp *newrelic.Application) (*Application, error) {
	appCtx := Bootstrap(ctx, cfg.GCSBucket, cfg.GCSUploadURLBase, cfg.FirebaseProjectID, logger, nrApp)
	store := appCtx.Store

	dispatcher, closeDispatcher, err := newWorkerDispatcher(ctx, cfg, logger, nrApp)
	if err != nil {
		return nil, err
	}

	objectMetadata := storage.NewObjectMetadataFetcher(cfg.InternalGCSUploadBase, cfg.GCSBucket)
	objectStore := storage.NewDocumentObjectStore(cfg.InternalGCSUploadBase, cfg.GCSBucket)
	sourceURLBuilder := NewDocumentSourceURLBuilder(cfg.GCSBucket, cfg.InternalGCSUploadBase)
	imageURLIssuer := NewDocumentImageURLIssuer(cfg.GCSBucket, cfg.InternalGCSUploadBase)

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
		_ = closeDispatcher()
		return nil, fmt.Errorf("stripe provider init: %w", err)
	}

	billingSvc, err := application.NewBillingService(application.BillingServiceDeps{
		Accounts:     store,
		Usage:        store,
		Provider:     stripeProvider,
		Logger:       logger,
		RequireUsage: requiresBilling(cfg.Env),
	})
	if err != nil {
		_ = closeDispatcher()
		return nil, fmt.Errorf("api.billing_init: %w", err)
	}

	documentSvc := application.NewDocumentService(application.DocumentServiceDeps{
		Repo:             store,
		Jobs:             store,
		Accounts:         store,
		LifecycleRepo:    store,
		Workspaces:       store,
		Tree:             store,
		Transactor:       store,
		SourceURLBuilder: sourceURLBuilder,
		ImageURLIssuer:   imageURLIssuer,
		ObjectMetadata:   objectMetadata,
		ObjectStore:      objectStore,
		Dispatcher:       dispatcher,
		Notifier:         appCtx.Notifier,
		Logger:           logger,
		NRApp:            nrApp,
	})
	itemSvc := application.NewItemService(application.ItemServiceDeps{Repo: store, Workspaces: store, Logger: logger})
	workspaceSvc := application.NewWorkspaceService(application.WorkspaceServiceDeps{Accounts: store, Workspaces: store, Users: store, Logger: logger})
	userSvc := application.NewUserService(application.UserServiceDeps{Users: store, Accounts: store, Billing: billingSvc, Logger: logger})
	treeSvc := application.NewTreeService(application.TreeServiceDeps{Tree: store, Workspaces: store, Logger: logger})
	devSeedSvc := application.NewDevSeedService(application.DevSeedServiceDeps{Accounts: store, Workspaces: store, Tree: store, Items: store})

	authenticator, err := apiauth.NewFirebaseAuthenticator(apiauth.FirebaseAuthenticatorConfig{
		ProjectID:        cfg.FirebaseProjectID,
		ServiceToken:     cfg.Auth.ServiceToken,
		AdminEmailsCSV:   cfg.Auth.AdminEmailsCSV,
		AllowedEmailsCSV: cfg.Auth.AllowedEmailsCSV,
	})
	if err != nil {
		_ = closeDispatcher()
		return nil, fmt.Errorf("authenticator init: %w", err)
	}

	mux := http.NewServeMux()
	connectOptions := append(
		observability.ConnectHandlerOptions(nrApp),
		observability.MaskInternalErrorsHandlerOptions(logger)...,
	)
	mux.Handle(appv1connect.NewDocumentServiceHandler(handler.NewDocumentHandler(documentSvc, store), connectOptions...))
	mux.Handle(appv1connect.NewTreeServiceHandler(handler.NewTreeHandler(treeSvc), connectOptions...))
	mux.Handle(appv1connect.NewItemServiceHandler(handler.NewItemHandler(itemSvc), connectOptions...))
	mux.Handle(appv1connect.NewWorkspaceServiceHandler(handler.NewWorkspaceHandler(workspaceSvc), connectOptions...))
	mux.Handle(appv1connect.NewUserServiceHandler(handler.NewUserHandler(userSvc), connectOptions...))
	mux.Handle(appv1connect.NewJobServiceHandler(handler.NewJobHandler(store, store, store, store, store, logger), connectOptions...))
	mux.Handle(appv1connect.NewBillingServiceHandler(handler.NewBillingHandler(billingSvc), connectOptions...))
	mux.HandleFunc("/stripe/webhook", handler.NewBillingWebhookHTTPHandler(billingSvc, logger))
	if devSeedEnabled(cfg.Env) {
		mux.Handle("POST /dev/seed-workspace", handler.NewDevSeedHTTPHandler(devSeedSvc, true))
	}
	mux.HandleFunc("GET /health", healthHandler(store, cfg.ReadinessKey, cfg.ReadinessMonitorKey))
	registerMaintenanceSweep(mux, cfg, documentSvc, logger)

	var h http.Handler = mux
	h = apimiddleware.WithAuth(authenticator, logger, h)
	h = apimiddleware.CORS(cfg.CORSAllowedOrigins, h)
	h = apimiddleware.SecurityHeaders(h)
	h = httpmiddleware.Logger(logger, h)
	h = httpmiddleware.Recover(logger, h)

	return &Application{Handler: h, Address: ":" + cfg.Port, close: closeDispatcher}, nil
}

func newWorkerDispatcher(ctx context.Context, cfg config.API, logger *slog.Logger, nrApp *newrelic.Application) (application.WorkerDispatcher, func() error, error) {
	if cfg.WorkerDispatch.CloudTasksQueue == "" {
		return apiworker.NewHTTPDispatcher(cfg.WorkerBaseURL, logger, observability.ConnectClientOptions(nrApp)...), func() error { return nil }, nil
	}
	dispatcher, err := apiworker.NewCloudTasksDispatcher(ctx, apiworker.CloudTasksDispatcherConfig{
		QueuePath:        cfg.WorkerDispatch.CloudTasksQueue,
		DispatchURL:      cfg.WorkerDispatch.DispatchURL,
		InvokerSA:        cfg.WorkerDispatch.InvokerSA,
		OIDCAudience:     cfg.WorkerDispatch.OIDCAudience,
		DispatchDeadline: cfg.WorkerDispatch.DispatchDeadline,
		Logger:           logger,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("cloud tasks dispatcher init: %w", err)
	}
	return dispatcher, dispatcher.Close, nil
}

// registerMaintenanceSweep mounts the Cloud Scheduler-driven housekeeping route
// (auto-resume of failed jobs + the stuck-job sweep that feeds the New Relic
// JobStuck alert). Both used to be in-process goroutines, which only works when
// Cloud Run keeps CPU allocated outside requests — i.e. when you pay for an
// always-on instance. Driving them from Cloud Scheduler lets the service run
// with cpu_idle=true and scale to zero between sweeps.
//
// Re-emitting the same job_id on every sweep is fine: the NR alert uses
// uniqueCount(job_id), so repeats within the window collapse to one.
//
// The route is only mounted when the scheduler identity is configured, so local
// and test runs (and any environment that has not been applied yet) expose no
// unauthenticated surface. It stays outside the Firebase auth middleware —
// see isAuthExempt — because it carries a Google OIDC token instead.
func registerMaintenanceSweep(mux *http.ServeMux, cfg config.API, documentSvc *application.DocumentService, logger *slog.Logger) {
	// Unset is the normal state locally and in tests, so this is not a warning.
	// A half-configured deployment is, and falls through to the error below.
	if strings.TrimSpace(cfg.Maintenance.SchedulerServiceAccountsCSV) == "" {
		logger.Info("maintenance.sweep_disabled", "reason", "SYNTHIFY_MAINTENANCE_SCHEDULER_SERVICE_ACCOUNTS is empty")
		return
	}
	schedulerAuth, err := apiauth.NewSchedulerAuthenticator(apiauth.SchedulerAuthenticatorConfig{
		Audience:                  cfg.Maintenance.OIDCAudience,
		AllowedServiceAccountsCSV: cfg.Maintenance.SchedulerServiceAccountsCSV,
	})
	if err != nil {
		logger.Error("maintenance.sweep_disabled", "error", err.Error())
		return
	}
	mux.Handle(apimiddleware.MaintenanceSweepPath, handler.NewMaintenanceHTTPHandler(documentSvc, schedulerAuth, logger))
	logger.Info("maintenance.sweep_enabled", "path", apimiddleware.MaintenanceSweepPath)
}

type readinessChecker interface {
	CheckReadiness(context.Context) error
}

func healthHandler(store any, readinessKey, readinessMonitorKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ready") != "1" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}

		provided := r.Header.Get("X-Synthify-Readiness-Key")
		if provided == "" {
			provided = r.URL.Query().Get("key")
		}
		if !constantTimeEqual(provided, readinessKey) && !constantTimeEqual(provided, readinessMonitorKey) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if checker, ok := store.(readinessChecker); ok {
			if err := checker.CheckReadiness(r.Context()); err != nil {
				http.Error(w, "not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

func constantTimeEqual(left, right string) bool {
	leftSum := sha256.Sum256([]byte(left))
	rightSum := sha256.Sum256([]byte(right))
	return subtle.ConstantTimeCompare(leftSum[:], rightSum[:]) == 1
}

func devSeedEnabled(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "local", "dev", "development", "test":
		return true
	default:
		return false
	}
}

func requiresBilling(env string) bool {
	return env == "production" || env == "staging"
}
