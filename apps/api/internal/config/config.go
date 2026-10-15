// Package config provides configuration structs and environment variable loaders
// for the Synthify API service, storage buckets, worker dispatching, authentication,
// Stripe billing, and observability.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/synthify/backend/internal/platform/storage"
)

// API holds all runtime configuration settings required by the Synthify API server.
type API struct {
	Port                     string
	Env                      string
	ReadinessKey             string
	ReadinessMonitorKey      string
	CORSAllowedOrigins       string
	GCSBucket                string
	GCSUploadURLBase         string
	InternalGCSUploadBase    string
	FirebaseProjectID        string
	FirebaseAuthEmulatorHost string
	WorkerBaseURL            string
	WorkerDispatch           WorkerDispatch
	Auth                     Auth
	Stripe                   *Stripe
	Billing                  Billing
	NewRelic                 NewRelic
	Chat                     Chat
}

// Chat configures the grounded-answer client for workspace chat. When no key
// and no Vertex project are set, bootstrap falls back to a non-model extractive
// answerer outside production and fails startup inside it.
type Chat struct {
	GeminiAPIKey   string
	GeminiModel    string
	VertexProject  string
	VertexLocation string
}

// WorkerDispatch controls how the API hands jobs to the worker. When
// CloudTasksQueue is set the API enqueues onto Cloud Tasks (push delivery
// to the dispatch URL); otherwise it falls back to the legacy synchronous
// Connect dispatcher driven by WorkerBaseURL.
//
// Cloud Tasks variant is preferred in stage/prod because Cloud Tasks
// handles cold-start retries and decouples the API response from worker
// startup latency.
type WorkerDispatch struct {
	CloudTasksQueue string // "projects/X/locations/Y/queues/Z"
	DispatchURL     string // "https://worker-host/internal/dispatch-job"
	InvokerSA       string // service account Cloud Tasks impersonates to mint OIDC tokens
	OIDCAudience    string // optional; defaults to scheme+host of DispatchURL
	// DispatchDeadline is how long Cloud Tasks waits for the worker to respond
	// to one attempt. It must be >= the worker's Cloud Run request timeout, or a
	// retry can overlap a still-running attempt and double-dispatch the job.
	// Set per-task at enqueue time (the queue resource has no such field). Fed
	// from terraform off the same worker timeout so the two never drift.
	DispatchDeadline time.Duration
}

// Auth defines credentials and access control lists for internal and external API callers.
type Auth struct {
	ServiceToken     string
	AdminEmailsCSV   string
	AllowedEmailsCSV string
}

// Stripe encapsulates API keys, pricing plan IDs, meter events, and webhook secrets for Stripe integration.
type Stripe struct {
	SecretKey        string
	WebhookSecret    string
	ProPriceIDJPY    string
	ProPriceIDUSD    string
	DefaultCurrency  string
	MeterInputEvent  string
	MeterOutputEvent string
	APIBase          string
	APIVersion       string
}

// Billing holds redirect URLs for Stripe Checkout and Customer Portal flows.
type Billing struct {
	SuccessURL      string
	CancelURL       string
	PortalReturnURL string
}

// NewRelic configures application telemetry and APM licensing credentials.
type NewRelic struct {
	AppName    string
	LicenseKey string
}

// Store holds database connection parameters and connection pool tuning parameters.
type Store struct {
	DatabaseDSN    string
	DBMaxOpenConns int
	DBMaxIdleConns int
}

// loadStripeConfig initializes Stripe configuration from environment variables,
// returning nil in local environments if STRIPE_SECRET_KEY is omitted.
func loadStripeConfig(env string) *Stripe {
	secret := os.Getenv("STRIPE_SECRET_KEY")
	if env == "local" && secret == "" {
		return nil
	}
	return &Stripe{
		SecretKey:        secret,
		WebhookSecret:    os.Getenv("STRIPE_WEBHOOK_SECRET"),
		ProPriceIDJPY:    os.Getenv("STRIPE_PRO_PRICE_ID_JPY"),
		ProPriceIDUSD:    os.Getenv("STRIPE_PRO_PRICE_ID_USD"),
		DefaultCurrency:  get("STRIPE_DEFAULT_CURRENCY", "jpy"),
		MeterInputEvent:  os.Getenv("STRIPE_METER_INPUT_EVENT"),
		MeterOutputEvent: os.Getenv("STRIPE_METER_OUTPUT_EVENT"),
		APIBase:          get("STRIPE_API_BASE", "https://api.stripe.com"),
		APIVersion:       get("STRIPE_API_VERSION", "2025-06-30.basil"),
	}
}

// LoadAPI loads and validates all API server configuration settings from environment variables.
func LoadAPI() API {
	uploadBase := mustBaseURL("GCS_UPLOAD_URL_BASE", get("GCS_UPLOAD_URL_BASE", "http://127.0.0.1:4443"))
	env := get("ENV", "production")

	return API{
		Port:                     get("PORT", "8080"),
		Env:                      env,
		ReadinessKey:             os.Getenv("SYNTHIFY_READINESS_KEY"),
		ReadinessMonitorKey:      os.Getenv("SYNTHIFY_READINESS_MONITOR_KEY"),
		CORSAllowedOrigins:       get("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:5174,http://127.0.0.1:5174,http://localhost:3000,http://127.0.0.1:3000"),
		GCSBucket:                get("GCS_BUCKET", "synthify-uploads"),
		GCSUploadURLBase:         uploadBase,
		InternalGCSUploadBase:    mustBaseURL("INTERNAL_GCS_UPLOAD_URL_BASE", get("INTERNAL_GCS_UPLOAD_URL_BASE", uploadBase)),
		FirebaseProjectID:        os.Getenv("FIREBASE_PROJECT_ID"),
		FirebaseAuthEmulatorHost: os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"),
		WorkerBaseURL:            os.Getenv("WORKER_BASE_URL"),
		WorkerDispatch: WorkerDispatch{
			CloudTasksQueue:  os.Getenv("WORKER_CLOUDTASKS_QUEUE"),
			DispatchURL:      os.Getenv("WORKER_DISPATCH_URL"),
			InvokerSA:        os.Getenv("WORKER_INVOKER_SA"),
			OIDCAudience:     os.Getenv("WORKER_OIDC_AUDIENCE"),
			DispatchDeadline: getDurationSeconds("WORKER_DISPATCH_DEADLINE_SECONDS", defaultDispatchDeadline),
		},
		Auth: Auth{
			ServiceToken:     os.Getenv("SYNTHIFY_INTERNAL_SERVICE_TOKEN"),
			AdminEmailsCSV:   os.Getenv("SYNTHIFY_ADMIN_USER_EMAILS"),
			AllowedEmailsCSV: os.Getenv("SYNTHIFY_ALLOWED_USER_EMAILS"),
		},
		Stripe: loadStripeConfig(env),
		Billing: Billing{
			SuccessURL:      get("BILLING_SUCCESS_URL", "http://localhost:3000/workspaces?billing=success"),
			CancelURL:       get("BILLING_CANCEL_URL", "http://localhost:3000/workspaces?billing=cancel"),
			PortalReturnURL: get("BILLING_PORTAL_RETURN_URL", "http://localhost:3000/workspaces"),
		},
		NewRelic: NewRelic{
			AppName:    get("NEW_RELIC_APP_NAME", defaultNewRelicAppName("synthify-api")),
			LicenseKey: os.Getenv("NEW_RELIC_LICENSE_KEY"),
		},
		Chat: Chat{
			// Same env vars the worker reads, so a deployment configures the
			// model once rather than separately per service.
			GeminiAPIKey: firstNonEmpty(os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY")),
			GeminiModel:  get("GEMINI_MODEL", "gemini-3-flash-preview"),
			VertexProject: firstNonEmpty(
				os.Getenv("GCP_PROJECT"),
				os.Getenv("GOOGLE_CLOUD_PROJECT"),
				os.Getenv("GCP_PROJECT_ID"),
			),
			VertexLocation: get("VERTEX_LOCATION", "global"),
		},
	}
}

// firstNonEmpty returns the first non-empty string in the provided list.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// LoadStore loads database DSN and connection pool configuration from environment variables.
func LoadStore() Store {
	return Store{
		DatabaseDSN:    os.Getenv("DATABASE_DSN"),
		DBMaxOpenConns: getInt("DB_MAX_OPEN_CONNS", 0),
		DBMaxIdleConns: getInt("DB_MAX_IDLE_CONNS", 0),
	}
}

// defaultDispatchDeadline is the fallback when WORKER_DISPATCH_DEADLINE_SECONDS
// is unset (local / tests). 15m is the Cloud Tasks max for HTTP targets and
// comfortably exceeds the worker's default request timeout.
const defaultDispatchDeadline = 15 * time.Minute

// getDurationSeconds reads an integer-seconds env var, falling back rather than
// failing on missing / unparseable / non-positive values.
func getDurationSeconds(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs <= 0 {
		return fallback
	}
	return time.Duration(secs) * time.Second
}

// get retrieves an environment variable value or returns the fallback if empty.
func get(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// defaultNewRelicAppName derives the New Relic APM application name based on deployment environment.
func defaultNewRelicAppName(base string) string {
	env := os.Getenv("ENV")
	switch env {
	case "prod", "production":
		return base
	case "stage", "staging":
		return base + "-stage"
	case "", "dev", "development", "local", "test":
		return base + "-local"
	default:
		return base + "-" + env
	}
}

// getInt reads a positive integer from the environment, falling back to def
// when the variable is unset or not a positive integer.
func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// mustBaseURL validates that the specified URL string is an acceptable HTTP/HTTPS base URL,
// panicking if validation fails.
func mustBaseURL(name, value string) string {
	if err := storage.ValidateBaseURL(value); err != nil {
		panic(fmt.Sprintf("%s is invalid: %v", name, err))
	}
	return value
}
