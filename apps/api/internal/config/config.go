package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/synthify/backend/internal/platform/storage"
)

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
	Stripe                   Stripe
	Billing                  Billing
	NewRelic                 NewRelic
	Maintenance              Maintenance
}

// Maintenance identifies the Cloud Scheduler job allowed to drive the periodic
// housekeeping sweep. Both fields must be set for the endpoint to be mounted;
// an unset allowlist disables it rather than opening it up.
type Maintenance struct {
	// OIDCAudience the scheduler's token must carry. Terraform sets it to the
	// API's own Cloud Run URL.
	OIDCAudience string
	// SchedulerServiceAccountsCSV are the caller identities permitted to invoke
	// the sweep.
	SchedulerServiceAccountsCSV string
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

type Auth struct {
	ServiceToken     string
	AdminEmailsCSV   string
	AllowedEmailsCSV string
}

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

type Billing struct {
	SuccessURL      string
	CancelURL       string
	PortalReturnURL string
}

type NewRelic struct {
	AppName    string
	LicenseKey string
}

type Store struct {
	DatabaseDSN    string
	DBMaxOpenConns int
	DBMaxIdleConns int
}

func LoadAPI() API {
	uploadBase := mustBaseURL("GCS_UPLOAD_URL_BASE", get("GCS_UPLOAD_URL_BASE", "http://127.0.0.1:4443"))
	return API{
		Port:                     get("PORT", "8080"),
		Env:                      get("ENV", "production"),
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
		Stripe: Stripe{
			SecretKey:        os.Getenv("STRIPE_SECRET_KEY"),
			WebhookSecret:    os.Getenv("STRIPE_WEBHOOK_SECRET"),
			ProPriceIDJPY:    os.Getenv("STRIPE_PRO_PRICE_ID_JPY"),
			ProPriceIDUSD:    os.Getenv("STRIPE_PRO_PRICE_ID_USD"),
			DefaultCurrency:  get("STRIPE_DEFAULT_CURRENCY", "jpy"),
			MeterInputEvent:  os.Getenv("STRIPE_METER_INPUT_EVENT"),
			MeterOutputEvent: os.Getenv("STRIPE_METER_OUTPUT_EVENT"),
			APIBase:          get("STRIPE_API_BASE", "https://api.stripe.com"),
			APIVersion:       get("STRIPE_API_VERSION", "2025-06-30.basil"),
		},
		Billing: Billing{
			SuccessURL:      get("BILLING_SUCCESS_URL", "http://localhost:3000/workspaces?billing=success"),
			CancelURL:       get("BILLING_CANCEL_URL", "http://localhost:3000/workspaces?billing=cancel"),
			PortalReturnURL: get("BILLING_PORTAL_RETURN_URL", "http://localhost:3000/workspaces"),
		},
		NewRelic: NewRelic{
			AppName:    get("NEW_RELIC_APP_NAME", defaultNewRelicAppName("synthify-api")),
			LicenseKey: os.Getenv("NEW_RELIC_LICENSE_KEY"),
		},
		Maintenance: Maintenance{
			OIDCAudience:                os.Getenv("SYNTHIFY_MAINTENANCE_OIDC_AUDIENCE"),
			SchedulerServiceAccountsCSV: os.Getenv("SYNTHIFY_MAINTENANCE_SCHEDULER_SERVICE_ACCOUNTS"),
		},
	}
}

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

func get(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

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

func mustBaseURL(name, value string) string {
	if err := storage.ValidateBaseURL(value); err != nil {
		panic(fmt.Sprintf("%s is invalid: %v", name, err))
	}
	return value
}
