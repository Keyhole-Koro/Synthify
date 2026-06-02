package llm

import (
	"testing"

	"golang.org/x/time/rate"
)

func TestNewRateLimiterFromQuota(t *testing.T) {
	// gemini-3-flash-preview is RPM 5 in quotas.go; free-tier models use
	// burst=1 so they are smoothed across the whole minute.
	l := newRateLimiter("gemini-3-flash-preview")
	wantPerSec := rate.Limit(5.0 / 60.0)
	if l.Limit() != wantPerSec {
		t.Errorf("limit = %v, want %v", l.Limit(), wantPerSec)
	}
	if l.Burst() != 1 {
		t.Errorf("burst = %d, want 1", l.Burst())
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
	// Unknown models resolve to defaultQuota via QuotaFor.
	if l.Limit() != rate.Limit(float64(defaultQuota.RPM)/60.0) {
		t.Errorf("limit = %v, want default %v", l.Limit(), rate.Limit(float64(defaultQuota.RPM)/60.0))
	}
	if l.Burst() != 1 {
		t.Errorf("burst = %d, want 1", l.Burst())
	}
}
