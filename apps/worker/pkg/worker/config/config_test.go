package config

import (
	"strings"
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

func TestLoadWorker_FixtureModeIsLocalOnly(t *testing.T) {
	t.Setenv("GCS_FUSE_MOUNT_PATH", t.TempDir())
	t.Setenv("E2E_WORKER_FIXTURE", "true")
	t.Setenv("ENV", "local")
	if cfg := LoadWorker(); !cfg.FixtureMode {
		t.Fatal("FixtureMode = false, want true")
	}
	t.Setenv("ENV", "production")
	defer func() {
		if recover() == nil {
			t.Fatal("production fixture mode did not panic")
		}
	}()
	_ = LoadWorker()
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

func TestLoadLLM_DefaultsToGemini(t *testing.T) {
	t.Setenv("GCP_PROJECT", "test-project")
	t.Setenv("LLM_PROVIDER", "")

	cfg := LoadLLM()
	if cfg.Provider != LLMProviderGemini {
		t.Fatalf("Provider = %q, want %q", cfg.Provider, LLMProviderGemini)
	}
	if err := cfg.ValidateProcessProvider(); err != nil {
		t.Fatalf("ValidateProcessProvider() error = %v", err)
	}
}

func TestLLMValidateProcessProvider(t *testing.T) {
	validLocal := LLM{
		Provider:                    LLMProviderAntigravity,
		RuntimeEnv:                  "local",
		DeploymentMode:              "self-hosted",
		LocalProviderEndpoint:       "http://127.0.0.1:8787",
		LocalProviderTokenFile:      "/owner-only/token",
		LocalProviderRequestTimeout: time.Minute,
		LocalProviderCancelTimeout:  time.Second,
		LocalProviderHealthTimeout:  time.Second,
	}

	tests := []struct {
		name    string
		mutate  func(*LLM)
		wantErr string
	}{
		{name: "valid local", mutate: func(*LLM) {}},
		{name: "production fails closed", mutate: func(cfg *LLM) {
			cfg.RuntimeEnv = "production"
		}, wantErr: "disabled"},
		{name: "staging fails closed", mutate: func(cfg *LLM) {
			cfg.RuntimeEnv = "staging"
		}, wantErr: "disabled"},
		{name: "self hosted required", mutate: func(cfg *LLM) {
			cfg.DeploymentMode = ""
		}, wantErr: "DEPLOYMENT_MODE=self-hosted"},
		{name: "loopback required", mutate: func(cfg *LLM) {
			cfg.LocalProviderEndpoint = "https://provider.example.com"
		}, wantErr: "loopback"},
		{name: "token file required", mutate: func(cfg *LLM) {
			cfg.LocalProviderTokenFile = ""
		}, wantErr: "TOKEN_FILE"},
		{name: "unknown provider", mutate: func(cfg *LLM) {
			cfg.Provider = "other"
		}, wantErr: "unsupported"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validLocal
			test.mutate(&cfg)
			err := cfg.ValidateProcessProvider()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateProcessProvider() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateProcessProvider() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateLoopbackEndpoint(t *testing.T) {
	for _, endpoint := range []string{
		"http://127.0.0.1:8787",
		"http://[::1]:8787",
		"https://localhost:8787",
	} {
		if err := validateLoopbackEndpoint(endpoint); err != nil {
			t.Errorf("validateLoopbackEndpoint(%q) error = %v", endpoint, err)
		}
	}

	for _, endpoint := range []string{
		"",
		"127.0.0.1:8787",
		"file:///tmp/provider.sock",
		"http://10.0.0.1:8787",
		"http://127.0.0.1:8787/rpc",
		"http://user:pass@127.0.0.1:8787",
	} {
		if err := validateLoopbackEndpoint(endpoint); err == nil {
			t.Errorf("validateLoopbackEndpoint(%q) unexpectedly passed", endpoint)
		}
	}
}
