package llm

import (
	"testing"

	"golang.org/x/time/rate"
)

func TestNewRateLimiterFromQuota(t *testing.T) {
	// gemini-3-flash-preview is RPM 10 in quotas.go.
	l := newRateLimiter("gemini-3-flash-preview")
	wantPerSec := rate.Limit(10.0 / 60.0)
	if l.Limit() != wantPerSec {
		t.Errorf("limit = %v, want %v", l.Limit(), wantPerSec)
	}
	if l.Burst() != 10 {
		t.Errorf("burst = %d, want 10", l.Burst())
	}
}

func TestNewRateLimiterBurstCappedForHighRPM(t *testing.T) {
	// gemini-2.5-flash is RPM 300; burst must stay capped at 10 so we never
	// release a 300-call burst in one instant.
	l := newRateLimiter("gemini-2.5-flash")
	if l.Burst() != 10 {
		t.Errorf("burst = %d, want capped at 10", l.Burst())
	}
	if l.Limit() != rate.Limit(300.0/60.0) {
		t.Errorf("limit = %v, want %v", l.Limit(), rate.Limit(300.0/60.0))
	}
}

func TestNewRateLimiterUnknownModelUsesDefault(t *testing.T) {
	l := newRateLimiter("totally-unknown-model")
	// Unknown models resolve to defaultQuota (RPM 10) via QuotaFor.
	if l.Limit() != rate.Limit(float64(defaultQuota.RPM)/60.0) {
		t.Errorf("limit = %v, want default %v", l.Limit(), rate.Limit(float64(defaultQuota.RPM)/60.0))
	}
}
