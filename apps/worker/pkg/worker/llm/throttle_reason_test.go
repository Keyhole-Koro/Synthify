package llm

import (
	"errors"
	"testing"

	"google.golang.org/genai"
)

func TestThrottleReasonFromAPIError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"429 code", genai.APIError{Code: 429}, "rate_limited"},
		{"resource exhausted status", genai.APIError{Status: "RESOURCE_EXHAUSTED"}, "rate_limited"},
		{"deadline exceeded", genai.APIError{Code: 504}, "timeout"},
		{"unavailable status", genai.APIError{Status: "UNAVAILABLE"}, "upstream_5xx"},
		{"internal 500", genai.APIError{Code: 500}, "upstream_5xx"},
		{"unknown 4xx", genai.APIError{Code: 400}, "other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := throttleReason(tc.err); got != tc.want {
				t.Errorf("throttleReason(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func TestThrottleReasonFromWrappedString(t *testing.T) {
	// genai sometimes wraps the failure as a plain string rather than APIError.
	cases := map[string]string{
		"rpc error: code = ... Error 429 quota": "rate_limited",
		"RESOURCE_EXHAUSTED: per-minute limit":  "rate_limited",
		"Error 504 DEADLINE_EXCEEDED":           "timeout",
		"UNAVAILABLE: backend overloaded":       "upstream_5xx",
		"Error 503 Service Unavailable":         "upstream_5xx",
		"some other unclassified failure":       "other",
	}
	for msg, want := range cases {
		if got := throttleReason(errors.New(msg)); got != want {
			t.Errorf("throttleReason(%q) = %q, want %q", msg, got, want)
		}
	}
}
