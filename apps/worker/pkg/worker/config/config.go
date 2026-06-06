package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
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
	GeminiModel    string
	APIKey         string
	LogPayload     bool
	GCPProject     string
	VertexLocation string
}

// defaultVertexLocation targets Vertex AI's global Gemini endpoint. Preview
// Gemini models can be unavailable in regional endpoints even when Cloud Run is
// deployed there, so production should not infer Vertex location from runtime
// region unless explicitly overridden.
const defaultVertexLocation = "global"

func LoadWorker() Worker {
	return Worker{
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
			AppName:    get("NEW_RELIC_APP_NAME", "synthify-worker"),
			LicenseKey: os.Getenv("NEW_RELIC_LICENSE_KEY"),
		},
		RequestTimeout: getDurationSeconds("WORKER_REQUEST_TIMEOUT_SECONDS", defaultRequestTimeout),
	}
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
		GeminiModel:    get("GEMINI_MODEL", "gemini-3.5-flash"),
		APIKey:         util.FirstNonEmpty(os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY")),
		LogPayload:     os.Getenv("LOG_LLM_PAYLOAD") == "true",
		GCPProject:     project,
		VertexLocation: location,
	}
}

// Enabled reports whether Vertex AI inference can run. The worker can't reach
// the genai backend without a project ID, so init must fail fast in that case;
// location falls back to a constant so it is never the blocker.
func (c LLM) Enabled() bool {
	return c.GCPProject != "" || c.APIKey != ""
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
