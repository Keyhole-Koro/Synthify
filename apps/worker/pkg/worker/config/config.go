package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/synthify/backend/internal/platform/storage"
	"github.com/synthify/backend/internal/platform/util"
)

type Worker struct {
	Port                     string
	ReadinessKey             string
	GCSBucket                string
	GCSUploadURLBase         string
	FirebaseProjectID        string
	FirebaseAuthEmulatorHost string
	GCSFuseMountPath         string
	APIBaseURL               string
	InternalServiceToken     string
	NewRelic                 NewRelic
}

type NewRelic struct {
	AppName    string
	LicenseKey string
}

type Store struct {
	DatabaseDSN string
}

type LLM struct {
	GeminiModel    string
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
	}
}

func LoadStore() Store {
	return Store{DatabaseDSN: os.Getenv("DATABASE_DSN")}
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
		GeminiModel:    get("GEMINI_MODEL", "gemini-3-flash-preview"),
		LogPayload:     os.Getenv("LOG_LLM_PAYLOAD") == "true",
		GCPProject:     project,
		VertexLocation: location,
	}
}

// Enabled reports whether Vertex AI inference can run. The worker can't reach
// the genai backend without a project ID, so init must fail fast in that case;
// location falls back to a constant so it is never the blocker.
func (c LLM) Enabled() bool {
	return c.GCPProject != ""
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
