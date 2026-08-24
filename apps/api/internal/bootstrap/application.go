package bootstrap

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/synthify/backend/apps/api/internal/application"
	apiauth "github.com/synthify/backend/apps/api/internal/auth"
	"github.com/synthify/backend/apps/api/internal/config"
	"github.com/synthify/backend/apps/api/internal/handler"
	apichat "github.com/synthify/backend/apps/api/internal/infrastructure/chat"
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

	var stripeProvider application.BillingProvider
	if cfg.Stripe != nil {
		sp, err := stripe.NewProvider(stripe.Config{
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
		stripeProvider = sp
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
	startAutoResume(documentSvc, logger)
	startStuckJobMonitor(documentSvc, logger)

	itemSvc := application.NewItemService(application.ItemServiceDeps{Repo: store, Workspaces: store, Logger: logger})
	workspaceSvc := application.NewWorkspaceService(application.WorkspaceServiceDeps{Accounts: store, Workspaces: store, Users: store, Logger: logger})
	userSvc := application.NewUserService(application.UserServiceDeps{Users: store, Accounts: store, Billing: billingSvc, Logger: logger})
	treeSvc := application.NewTreeService(application.TreeServiceDeps{Tree: store, Workspaces: store, Logger: logger})
	devSeedSvc := application.NewDevSeedService(application.DevSeedServiceDeps{
		Accounts:   store,
		Workspaces: store,
		Tree:       store,
		Items:      store,
		Documents:  store,
		Chunks:     store,
		Jobs:       store,
	})

	chatAnswerer, err := newChatAnswerer(ctx, cfg, logger)
	if err != nil {
		_ = closeDispatcher()
		return nil, fmt.Errorf("chat answerer init: %w", err)
	}
	workspaceChatSvc := application.NewWorkspaceChatService(application.WorkspaceChatServiceDeps{
		Repo:       store,
		Workspaces: store,
		Answerer:   chatAnswerer,
		Logger:     logger,
	})

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
	mux.Handle(appv1connect.NewWorkspaceChatServiceHandler(handler.NewWorkspaceChatHandler(workspaceChatSvc), connectOptions...))
	mux.HandleFunc("/stripe/webhook", handler.NewBillingWebhookHTTPHandler(billingSvc, logger))
	if devSeedEnabled(cfg.Env) {
		mux.Handle("POST /dev/seed-workspace", handler.NewDevSeedHTTPHandler(devSeedSvc, true))
	}
	mux.HandleFunc("GET /health", healthHandler(store, cfg.ReadinessKey, cfg.ReadinessMonitorKey))

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

func startAutoResume(documentSvc *application.DocumentService, logger *slog.Logger) {
	go func() {
		time.Sleep(15 * time.Second)
		resumeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		resumed, err := documentSvc.AutoResumeFailedJobs(resumeCtx)
		if err != nil {
			logger.Error("job.auto_resume_scan_failed", "error", err.Error())
			return
		}
		if resumed > 0 {
			logger.Info("job.auto_resume_completed", "resumed", resumed)
		}
	}()
}

// stuckJobScanInterval is how often the stuck-job sweep runs. Re-emitting the
// same job_id every interval is fine: the NR alert uses uniqueCount(job_id), so
// repeats within the window collapse to one and running on several API
// instances does not inflate the count.
const stuckJobScanInterval = 5 * time.Minute

// startStuckJobMonitor periodically sweeps for jobs wedged in QUEUED/RUNNING and
// emits JobStuck events for New Relic to alert on. These never surface as
// JobFailed, so this sweep is the only signal that a job silently stopped
// progressing.
func startStuckJobMonitor(documentSvc *application.DocumentService, logger *slog.Logger) {
	go func() {
		ticker := time.NewTicker(stuckJobScanInterval)
		defer ticker.Stop()
		for range ticker.C {
			scanCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
			stuck, err := documentSvc.ReportStuckJobs(scanCtx)
			cancel()
			if err != nil {
				logger.Error("job.stuck_scan_failed", "error", err.Error())
				continue
			}
			if stuck > 0 {
				logger.Warn("job.stuck_scan_found", "count", stuck)
			}
		}
	}()
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

// newChatAnswerer selects the grounded-answer client for workspace chat.
//
// With a Gemini key (or Vertex project) configured it returns the real client.
// Without one it returns the extractive answerer, but only outside production —
// in production a missing model configuration is a startup failure, not a
// silent downgrade to quoting chunks back at the user.
func newChatAnswerer(ctx context.Context, cfg config.API, logger *slog.Logger) (application.ChatAnswerer, error) {
	modelConfigured := cfg.Chat.GeminiAPIKey != "" || cfg.Chat.VertexProject != ""

	if modelConfigured {
		answerer, err := apichat.NewGeminiAnswerer(ctx, apichat.GeminiAnswererConfig{
			APIKey:   cfg.Chat.GeminiAPIKey,
			Project:  cfg.Chat.VertexProject,
			Location: cfg.Chat.VertexLocation,
			Model:    cfg.Chat.GeminiModel,
		})
		if err == nil {
			logger.Info("chat.answerer_gemini", "model", cfg.Chat.GeminiModel)
			return answerer, nil
		}
		// A configured project is not the same as usable credentials. The local
		// compose stack sets GCP_PROJECT_ID=synthify-local as a placeholder, so
		// the Vertex backend gets picked and then fails looking up ADC. Outside
		// production that must not stop the API from booting.
		if requiresBilling(cfg.Env) {
			return nil, err
		}
		logger.Warn("chat.answerer_extractive",
			"env", cfg.Env,
			"reason", "gemini client init failed",
			"error", err.Error(),
			"effect", "chat quotes retrieved chunks instead of generating an answer",
		)
		return apichat.NewExtractiveAnswerer(), nil
	}

	if requiresBilling(cfg.Env) {
		return nil, fmt.Errorf("workspace chat needs GEMINI_API_KEY (or GOOGLE_API_KEY / GCP_PROJECT) in %s", cfg.Env)
	}
	logger.Warn("chat.answerer_extractive",
		"env", cfg.Env,
		"reason", "no GEMINI_API_KEY / GOOGLE_API_KEY / GCP_PROJECT configured",
		"effect", "chat quotes retrieved chunks instead of generating an answer",
	)
	return apichat.NewExtractiveAnswerer(), nil
}
