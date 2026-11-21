package config

import (
	"testing"
	"time"
)

func TestLoadAPI_DeletedWorkspaceRetention(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
	}{
		{"", 30 * 24 * time.Hour},
		{"7", 7 * 24 * time.Hour},
		{" 90 ", 90 * 24 * time.Hour},
		{"0", 0},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			t.Setenv("DELETED_WORKSPACE_RETENTION_DAYS", tc.raw)
			if got := LoadAPI().DataRetention.DeletedWorkspaces; got != tc.want {
				t.Fatalf("DeletedWorkspaces = %v, want %v", got, tc.want)
			}
		})
	}
}

// A typo must not silently purge on a schedule nobody chose.
func TestLoadAPI_RejectsMalformedRetention(t *testing.T) {
	for _, raw := range []string{"30d", "-1", "1.5"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("DELETED_WORKSPACE_RETENTION_DAYS", raw)
			defer func() {
				if recover() == nil {
					t.Fatalf("LoadAPI accepted DELETED_WORKSPACE_RETENTION_DAYS=%q", raw)
				}
			}()
			LoadAPI()
		})
	}
}
