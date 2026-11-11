package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBudgetPeriodStart(t *testing.T) {
	mustTime := func(s string) time.Time {
		t.Helper()
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	cases := []struct {
		name        string
		start, end  string
		now         string
		wantStartAt string
	}{
		{
			name:        "uses the Stripe period that contains now",
			start:       "2026-10-07T03:00:00Z",
			end:         "2026-11-07T03:00:00Z",
			now:         "2026-10-15T12:00:00Z",
			wantStartAt: "2026-10-07T03:00:00Z",
		},
		{
			name:        "falls back to the JST calendar month without a period",
			now:         "2026-10-15T12:00:00Z",
			wantStartAt: "2026-09-30T15:00:00Z", // 2026-10-01 00:00 JST
		},
		{
			name:        "uses the JST month even when UTC is still in the previous month",
			now:         "2026-10-31T16:00:00Z", // 2026-11-01 01:00 JST
			wantStartAt: "2026-10-31T15:00:00Z", // 2026-11-01 00:00 JST
		},
		{
			name:        "falls back when the recorded period has ended (renewal not received yet)",
			start:       "2026-09-07T03:00:00Z",
			end:         "2026-10-07T03:00:00Z",
			now:         "2026-10-15T12:00:00Z",
			wantStartAt: "2026-09-30T15:00:00Z",
		},
		{
			name:        "falls back when only the end is known",
			end:         "2026-11-07T03:00:00Z",
			now:         "2026-10-15T12:00:00Z",
			wantStartAt: "2026-09-30T15:00:00Z",
		},
		{
			name:        "the period end itself belongs to the next period",
			start:       "2026-10-07T03:00:00Z",
			end:         "2026-11-07T03:00:00Z",
			now:         "2026-11-07T03:00:00Z",
			wantStartAt: "2026-10-31T15:00:00Z",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BudgetPeriodStart(tc.start, tc.end, mustTime(tc.now))
			assert.Equal(t, mustTime(tc.wantStartAt), got)
		})
	}
}

func TestBudgetExceeded(t *testing.T) {
	assert.False(t, BudgetExceeded(0, 1_000_000), "no budget")
	assert.False(t, BudgetExceeded(1000, 1000), "exactly at the limit is still within budget")
	assert.True(t, BudgetExceeded(1000, 1001))
}
