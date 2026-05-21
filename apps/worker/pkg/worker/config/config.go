package config

import (
	"fmt"
	"os"

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
	GeminiAPIKey string
	GeminiModel  string
	LogPayload   bool
}

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
	return LLM{
		GeminiAPIKey: util.FirstNonEmpty(os.Getenv("GEMINI_API_KEY"), os.Getenv("GOOGLE_API_KEY")),
		GeminiModel:  get("GEMINI_MODEL", "gemini-3-flash-preview"),
		LogPayload:   os.Getenv("LOG_LLM_PAYLOAD") == "true",
	}
}

func (c LLM) Enabled() bool {
	return c.GeminiAPIKey != ""
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
