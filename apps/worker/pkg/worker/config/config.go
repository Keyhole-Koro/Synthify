package config

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/synthify/backend/internal/platform/storage"
	"github.com/synthify/backend/internal/platform/util"
)

type Worker struct {
	Port                     string
	Env                      string
	ReadinessKey             string
	GCSBucket                string
	GCSUploadURLBase         string
	FirebaseProjectID        string
	FirebaseAuthEmulatorHost string
	GCSFuseMountPath         string
	APIBaseURL               string
	InternalServiceToken     string
	NewRelic                 NewRelic
	// RequestTimeout mirrors the Cloud Run request timeout (same env value feeds
	// both, set in terraform). The worker derives its own, slightly shorter
	// wall-clock budget from this so it can fail a job cleanly *before* Cloud Run
	// hard-cancels the request ctx — a hard cancel risks wedging the job in
	// RUNNING. See AgentBudget.
	RequestTimeout time.Duration
	FixtureMode    bool
}

// defaultRequestTimeout is the fallback when WORKER_REQUEST_TIMEOUT_SECONDS is
// unset (local / tests). Matches Cloud Run's own default so behaviour does not
// change silently when the env var is missing.
const defaultRequestTimeout = 300 * time.Second

// AgentBudget is the wall-clock the agent loop is allowed before the
// orchestrator aborts it itself. Kept at 90% of the request timeout so the abort
// + FAILED transition land inside the request, ahead of Cloud Run's hard cancel.
func (w Worker) AgentBudget() time.Duration {
	return time.Duration(float64(w.RequestTimeout) * 0.9)
}

// RequiresBilling reports whether this environment must have a fully wired
// billing pipeline. In production and staging a missing service token or
// API base URL silently drops usage events (no-op reporter) and never bills
// — so the worker must fail fast at startup instead. dev/local/test keep the
// lenient behaviour so they can run without the billing API.
func (w Worker) RequiresBilling() bool {
	switch w.Env {
	case "production", "staging":
		return true
	default:
		return false
	}
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

type LLM struct {
	GeminiModel                 string
	APIKey                      string
	LogPayload                  bool
	GCPProject                  string
	VertexLocation              string
	Provider                    LLMProvider
	RuntimeEnv                  string
	DeploymentMode              string
	LocalProviderEndpoint       string
	LocalProviderTokenFile      string
	LocalProviderRequestTimeout time.Duration
	LocalProviderCancelTimeout  time.Duration
	LocalProviderHealthTimeout  time.Duration
}

type LLMProvider string

const (
	LLMProviderGemini      LLMProvider = "gemini"
	LLMProviderAntigravity LLMProvider = "antigravity"
	LLMProviderCodex       LLMProvider = "codex"

	defaultLocalProviderRequestTimeout = 120 * time.Second
	defaultLocalProviderCancelTimeout  = 2 * time.Second
	defaultLocalProviderHealthTimeout  = 2 * time.Second
)

// defaultVertexLocation targets Vertex AI's global Gemini endpoint. Preview
// Gemini models can be unavailable in regional endpoints even when Cloud Run is
// deployed there, so production should not infer Vertex location from runtime
// region unless explicitly overridden.
const defaultVertexLocation = "global"

func LoadWorker() Worker {
	cfg := Worker{
		Port:                     get("PORT", "8080"),
		Env:                      get("ENV", "production"),
		ReadinessKey:             os.Getenv("SYNTHIFY_READINESS_KEY"),
		GCSBucket:                get("GCS_BUCKET", "synthify-uploads"),
		GCSUploadURLBase:         mustBaseURL("GCS_UPLOAD_URL_BASE", get("GCS_UPLOAD_URL_BASE", "http://127.0.0.1:4443")),
		FirebaseProjectID:        os.Getenv("FIREBASE_PROJECT_ID"),
		FirebaseAuthEmulatorHost: os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"),
		GCSFuseMountPath:         mustMountPath("GCS_FUSE_MOUNT_PATH", os.Getenv("GCS_FUSE_MOUNT_PATH")),
		APIBaseURL:               get("NEXT_PUBLIC_API_BASE_URL", get("API_BASE_URL", "http://127.0.0.1:8080")),
		InternalServiceToken:     os.Getenv("INTERNAL_WORKER_TOKEN"),
		NewRelic: NewRelic{
			AppName:    get("NEW_RELIC_APP_NAME", defaultNewRelicAppName("synthify-worker")),
			LicenseKey: os.Getenv("NEW_RELIC_LICENSE_KEY"),
		},
		RequestTimeout: getDurationSeconds("WORKER_REQUEST_TIMEOUT_SECONDS", defaultRequestTimeout),
		FixtureMode:    os.Getenv("E2E_WORKER_FIXTURE") == "true",
	}
	if cfg.FixtureMode && cfg.Env != "local" && cfg.Env != "test" && cfg.Env != "development" && cfg.Env != "dev" {
		panic("E2E_WORKER_FIXTURE is only allowed in local/test/development")
	}
	return cfg
}

// getDurationSeconds reads an integer-seconds env var. A missing, empty, or
// unparseable value falls back rather than panicking, so a misconfigured env
// never blocks startup — the worker just runs on the default budget.
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

func LoadStore() Store {
	return Store{
		DatabaseDSN:    os.Getenv("DATABASE_DSN"),
		DBMaxOpenConns: getInt("DB_MAX_OPEN_CONNS", 0),
		DBMaxIdleConns: getInt("DB_MAX_IDLE_CONNS", 0),
	}
}

func LoadLLM() LLM {
	project := util.FirstNonEmpty(
		os.Getenv("GCP_PROJECT"),
		os.Getenv("GOOGLE_CLOUD_PROJECT"),
		os.Getenv("GCP_PROJECT_ID"),
	)
	location := get("VERTEX_LOCATION", defaultVertexLocation)
	if project == "" {
		mdProject := detectProjectFromMetadata()
		if project == "" {
			project = mdProject
		}
	}
	return LLM{
		GeminiModel:                 get("GEMINI_MODEL", "gemini-3.5-flash"),
		APIKey:                      util.FirstNonEmpty(os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY")),
		LogPayload:                  os.Getenv("LOG_LLM_PAYLOAD") == "true",
		GCPProject:                  project,
		VertexLocation:              location,
		Provider:                    LLMProvider(get("LLM_PROVIDER", string(LLMProviderGemini))),
		RuntimeEnv:                  os.Getenv("ENV"),
		DeploymentMode:              os.Getenv("DEPLOYMENT_MODE"),
		LocalProviderEndpoint:       os.Getenv("LOCAL_PROVIDER_ENDPOINT"),
		LocalProviderTokenFile:      os.Getenv("LOCAL_PROVIDER_TOKEN_FILE"),
		LocalProviderRequestTimeout: getDurationSeconds("LOCAL_PROVIDER_REQUEST_TIMEOUT_SECONDS", defaultLocalProviderRequestTimeout),
		LocalProviderCancelTimeout:  getDurationSeconds("LOCAL_PROVIDER_CANCEL_TIMEOUT_SECONDS", defaultLocalProviderCancelTimeout),
		LocalProviderHealthTimeout:  getDurationSeconds("LOCAL_PROVIDER_HEALTH_TIMEOUT_SECONDS", defaultLocalProviderHealthTimeout),
	}
}

// Enabled reports whether Vertex AI inference can run. The worker can't reach
// the genai backend without a project ID, so init must fail fast in that case;
// location falls back to a constant so it is never the blocker.
func (c LLM) Enabled() bool {
	return c.GCPProject != "" || c.APIKey != ""
}

func (c LLM) UsesLocalProvider() bool {
	return c.Provider == LLMProviderAntigravity || c.Provider == LLMProviderCodex
}

// ValidateProcessProvider enforces the local-provider deployment boundary.
// ADK and embeddings remain Gemini-backed; this selector affects only direct
// process-tool generation through llm.Client.
func (c LLM) ValidateProcessProvider() error {
	switch c.Provider {
	case LLMProviderGemini:
		return nil
	case LLMProviderAntigravity, LLMProviderCodex:
	default:
		return fmt.Errorf("unsupported LLM_PROVIDER %q", c.Provider)
	}

	switch c.RuntimeEnv {
	case "prod", "production", "stage", "staging":
		return fmt.Errorf("LLM_PROVIDER %q is disabled in %s", c.Provider, c.RuntimeEnv)
	}
	if c.DeploymentMode != "self-hosted" {
		return fmt.Errorf("LLM_PROVIDER %q requires DEPLOYMENT_MODE=self-hosted", c.Provider)
	}
	if strings.TrimSpace(c.LocalProviderTokenFile) == "" {
		return fmt.Errorf("LOCAL_PROVIDER_TOKEN_FILE is required")
	}
	if err := validateLoopbackEndpoint(c.LocalProviderEndpoint); err != nil {
		return fmt.Errorf("LOCAL_PROVIDER_ENDPOINT is invalid: %w", err)
	}
	if c.LocalProviderRequestTimeout <= 0 || c.LocalProviderCancelTimeout <= 0 || c.LocalProviderHealthTimeout <= 0 {
		return fmt.Errorf("local provider timeouts must be positive")
	}
	return nil
}

func validateLoopbackEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("must be an absolute URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("credentials, query, fragment, and path are not allowed")
	}
	host := u.Hostname()
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("host must be loopback")
		}
	}
	return nil
}

// detectProjectFromMetadata reads project from the Cloud Run metadata server
// so production deploys don't have to plumb GCP_PROJECT through terraform.
// Off-GCE callers (laptops, CI without ADC) get an empty string and must set an
// env var explicitly.
func detectProjectFromMetadata() string {
	if !metadata.OnGCE() {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if id, err := metadata.ProjectIDWithContext(ctx); err == nil {
		return id
	}
	return ""
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

func mustMountPath(name, value string) string {
	if value == "" {
		panic(fmt.Sprintf("%s is required: the worker reads source files from the gcsfuse mount", name))
	}
	if info, err := os.Stat(value); err != nil || !info.IsDir() {
		panic(fmt.Sprintf("%s %q is not a mounted directory: %v", name, value, err))
	}
	return value
}
