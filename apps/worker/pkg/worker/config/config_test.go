package config

import (
	"testing"
	"time"
)

func TestLoadWorker_RequestTimeoutFromEnv(t *testing.T) {
	t.Setenv("GCS_FUSE_MOUNT_PATH", t.TempDir()) // LoadWorker panics without a real mount dir
	t.Setenv("WORKER_REQUEST_TIMEOUT_SECONDS", "600")

	cfg := LoadWorker()
	if cfg.RequestTimeout != 600*time.Second {
		t.Errorf("RequestTimeout = %v, want 600s", cfg.RequestTimeout)
	}
	// Budget is 90% so the abort + FAILED transition land before Cloud Run's
	// hard cancel.
	if cfg.AgentBudget() != 540*time.Second {
		t.Errorf("AgentBudget = %v, want 540s", cfg.AgentBudget())
	}
}

func TestLoadWorker_RequestTimeoutDefaults(t *testing.T) {
	t.Setenv("GCS_FUSE_MOUNT_PATH", t.TempDir())
	t.Setenv("WORKER_REQUEST_TIMEOUT_SECONDS", "")

	cfg := LoadWorker()
	if cfg.RequestTimeout != defaultRequestTimeout {
		t.Errorf("RequestTimeout = %v, want default %v", cfg.RequestTimeout, defaultRequestTimeout)
	}
}

func TestLoadWorker_DefaultNewRelicAppNameIsLocalWhenEnvUnset(t *testing.T) {
	t.Setenv("GCS_FUSE_MOUNT_PATH", t.TempDir())
	t.Setenv("ENV", "")
	t.Setenv("NEW_RELIC_APP_NAME", "")

	cfg := LoadWorker()
	if cfg.NewRelic.AppName != "synthify-worker-local" {
		t.Fatalf("NewRelic.AppName = %q, want synthify-worker-local", cfg.NewRelic.AppName)
	}
}

func TestLoadWorker_DefaultNewRelicAppNameFollowsEnv(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"production", "production", "synthify-worker"},
		{"prod", "prod", "synthify-worker"},
		{"staging", "staging", "synthify-worker-stage"},
		{"stage", "stage", "synthify-worker-stage"},
		{"custom", "preview", "synthify-worker-preview"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GCS_FUSE_MOUNT_PATH", t.TempDir())
			t.Setenv("ENV", tc.env)
			t.Setenv("NEW_RELIC_APP_NAME", "")

			cfg := LoadWorker()
			if cfg.NewRelic.AppName != tc.want {
				t.Fatalf("NewRelic.AppName = %q, want %q", cfg.NewRelic.AppName, tc.want)
			}
		})
	}
}

func TestLoadWorker_NewRelicAppNameOverrideWins(t *testing.T) {
	t.Setenv("GCS_FUSE_MOUNT_PATH", t.TempDir())
	t.Setenv("ENV", "")
	t.Setenv("NEW_RELIC_APP_NAME", "custom-worker")

	cfg := LoadWorker()
	if cfg.NewRelic.AppName != "custom-worker" {
		t.Fatalf("NewRelic.AppName = %q, want custom-worker", cfg.NewRelic.AppName)
	}
}

func TestGetDurationSeconds(t *testing.T) {
	const fallback = 300 * time.Second
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"valid", "600", 600 * time.Second},
		{"empty falls back", "", fallback},
		{"non-numeric falls back", "abc", fallback},
		{"zero falls back", "0", fallback},
		{"negative falls back", "-5", fallback},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("X_TIMEOUT", tc.raw)
			if got := getDurationSeconds("X_TIMEOUT", fallback); got != tc.want {
				t.Errorf("getDurationSeconds(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}
