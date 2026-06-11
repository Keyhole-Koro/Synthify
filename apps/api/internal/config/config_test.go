package config

import "testing"

func TestLoadAPI_DefaultNewRelicAppNameIsLocalWhenEnvUnset(t *testing.T) {
	t.Setenv("ENV", "")
	t.Setenv("NEW_RELIC_APP_NAME", "")

	cfg := LoadAPI()
	if cfg.NewRelic.AppName != "synthify-api-local" {
		t.Fatalf("NewRelic.AppName = %q, want synthify-api-local", cfg.NewRelic.AppName)
	}
}

func TestLoadAPI_DefaultNewRelicAppNameFollowsEnv(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"production", "production", "synthify-api"},
		{"prod", "prod", "synthify-api"},
		{"staging", "staging", "synthify-api-stage"},
		{"stage", "stage", "synthify-api-stage"},
		{"custom", "preview", "synthify-api-preview"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENV", tc.env)
			t.Setenv("NEW_RELIC_APP_NAME", "")

			cfg := LoadAPI()
			if cfg.NewRelic.AppName != tc.want {
				t.Fatalf("NewRelic.AppName = %q, want %q", cfg.NewRelic.AppName, tc.want)
			}
		})
	}
}

func TestLoadAPI_NewRelicAppNameOverrideWins(t *testing.T) {
	t.Setenv("ENV", "")
	t.Setenv("NEW_RELIC_APP_NAME", "custom-api")

	cfg := LoadAPI()
	if cfg.NewRelic.AppName != "custom-api" {
		t.Fatalf("NewRelic.AppName = %q, want custom-api", cfg.NewRelic.AppName)
	}
}
