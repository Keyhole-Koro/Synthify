package llm

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"
	"golang.org/x/time/rate"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// RetryConfig defines the retry and backoff policy for the ADK model.LLM wrapper.
type RetryConfig struct {
	MaxAttempts int           // Total attempts including initial attempt
	BaseDelay   time.Duration // Base duration for exponential backoff
	MaxDelay    time.Duration // Maximum cap for backoff duration
	Jitter      float64       // Ratio in [0,1) applied as random jitter
}

// withDefaults fills unspecified retry parameters with sensible defaults.
func (c RetryConfig) withDefaults() RetryConfig {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 4
	}
	if c.BaseDelay <= 0 {
		c.BaseDelay = 1500 * time.Millisecond
	}
	if c.MaxDelay <= 0 {
		c.MaxDelay = 30 * time.Second
	}
	if c.Jitter < 0 {
		c.Jitter = 0
	}
	if c.Jitter > 1 {
		c.Jitter = 1
	}
	return c
}

// RetryingModel wraps model.LLM to catch 429, 5xx, and UNAVAILABLE errors and retry
// with exponential backoff and jitter. If any successful response was already yielded
// in streaming mode, subsequent errors are passed through without retrying.
//
// Additionally, RetryingModel enforces client-side rate limiting capped by the model's
// published RPM (QuotaFor). This smooths agent request bursts to prevent tripping
// tight Vertex AI preview quotas.
type RetryingModel struct {
	inner   model.LLM
	cfg     RetryConfig
	limiter *rate.Limiter
	logger  *slog.Logger
	nrApp   *newrelic.Application
}

// NewRetryingModel creates a retrying, rate-limited model.LLM decorator.
func NewRetryingModel(inner model.LLM, cfg RetryConfig, logger *slog.Logger, nrApp *newrelic.Application) *RetryingModel {
	if logger == nil {
		logger = slog.Default()
	}
	return &RetryingModel{
		inner:   inner,
		cfg:     cfg.withDefaults(),
		limiter: newRateLimiter(inner.Name()),
		logger:  logger,
		nrApp:   nrApp,
	}
}

// newRateLimiter builds a per-minute limiter from the model's reference RPM.
// Free/preview quotas are strict enough that even a small burst can consume
// the minute's whole budget and make the next agent step fail with 429. Keep
// burst=1 for low-RPM models so calls are smoothed across the full minute.
func newRateLimiter(model string) *rate.Limiter {
	rpm := QuotaFor(model).RPM
	if rpm <= 0 {
		rpm = defaultQuota.RPM
	}
	perSecond := rate.Limit(float64(rpm) / 60.0)
	burst := rpm
	if rpm <= 5 {
		burst = 1
	} else if burst > 10 {
		burst = 10
	}
	if burst < 1 {
		burst = 1
	}
	return rate.NewLimiter(perSecond, burst)
}

// Name returns the underlying model identifier.
func (m *RetryingModel) Name() string { return m.inner.Name() }

// GenerateContent executes model generation with rate limiting, retries, and New Relic telemetry.
func (m *RetryingModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		for attempt := 1; attempt <= m.cfg.MaxAttempts; attempt++ {
			yielded := false
			var finalErr error

			// Smooth the call rate under the model's RPM before every attempt
			// (the retry itself also consumes a token, so a backoff storm is
			// bounded by the limiter too). A cancelled context here surfaces as
			// the call error rather than a retryable failure.
			if err := m.limiter.Wait(ctx); err != nil {
				yield(nil, err)
				return
			}

			for resp, err := range m.inner.GenerateContent(ctx, req, stream) {
				if err == nil {
					if !yield(resp, nil) {
						return
					}
					yielded = true
					continue
				}
				finalErr = err
				break
			}

			if finalErr == nil {
				return
			}
			if yielded {
				// Failures following partial yields cannot be safely retried.
				yield(nil, finalErr)
				return
			}
			if !isRetryable(finalErr) || attempt == m.cfg.MaxAttempts {
				yield(nil, finalErr)
				return
			}

			delay := m.backoff(attempt, finalErr)
			m.logger.Warn("worker.llm_retry",
				"attempt", attempt,
				"max_attempts", m.cfg.MaxAttempts,
				"delay_ms", delay.Milliseconds(),
				"model", m.inner.Name(),
				"error", finalErr.Error(),
			)
			// Surface throttling to New Relic so sustained 429/quota pressure can
			// be alerted on before it degrades into failed jobs. The slog line
			// above only reaches Cloud Logging; retried-away 429s never otherwise
			// appear in NR. no-op when NR is disabled.
			m.recordThrottle(attempt, finalErr)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			}
		}
	}
}

// backoff calculates exponential backoff duration with jitter for the given attempt.
func (m *RetryingModel) backoff(attempt int, _ error) time.Duration {
	d := m.cfg.BaseDelay << (attempt - 1)
	if d <= 0 || d > m.cfg.MaxDelay {
		d = m.cfg.MaxDelay
	}
	if m.cfg.Jitter > 0 {
		// [-Jitter, +Jitter] * d
		j := (rand.Float64()*2 - 1) * m.cfg.Jitter
		d = time.Duration(float64(d) * (1 + j))
	}
	if d < 0 {
		d = 0
	}
	return d
}

// recordThrottle emits an LLMThrottled custom event for each retryable LLM
// failure. The rate of these events is the leading indicator of quota
// starvation — it climbs while retries are still masking the pressure, ahead
// of the JobFailed events that only fire once retries are exhausted.
func (m *RetryingModel) recordThrottle(attempt int, cause error) {
	if m.nrApp == nil {
		return
	}
	m.nrApp.RecordCustomEvent("LLMThrottled", map[string]any{
		"model":   m.inner.Name(),
		"attempt": attempt,
		"reason":  throttleReason(cause),
		"error":   cause.Error(),
	})
}

// throttleReason buckets a retryable error into a stable, low-cardinality
// label so alerts and dashboards can distinguish rate-limit (429/quota)
// pressure from upstream 5xx/unavailable blips.
func throttleReason(err error) string {
	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case 429:
			return "rate_limited"
		case 408, 504:
			return "timeout"
		case 500, 502, 503:
			return "upstream_5xx"
		}
		switch strings.ToUpper(apiErr.Status) {
		case "RESOURCE_EXHAUSTED":
			return "rate_limited"
		case "DEADLINE_EXCEEDED":
			return "timeout"
		case "UNAVAILABLE", "INTERNAL", "ABORTED":
			return "upstream_5xx"
		}
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "Error 429") || strings.Contains(s, "RESOURCE_EXHAUSTED"):
		return "rate_limited"
	case strings.Contains(s, "Error 504") || strings.Contains(s, "DEADLINE_EXCEEDED"):
		return "timeout"
	case strings.Contains(s, "UNAVAILABLE") ||
		strings.Contains(s, "Error 500") ||
		strings.Contains(s, "Error 502") ||
		strings.Contains(s, "Error 503"):
		return "upstream_5xx"
	}
	return "other"
}

// isRetryable determines whether a genai.APIError or wrapped error string represents a retryable condition.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var apiErr genai.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case 408, 429, 500, 502, 503, 504:
			return true
		}
		switch strings.ToUpper(apiErr.Status) {
		case "RESOURCE_EXHAUSTED", "UNAVAILABLE", "DEADLINE_EXCEEDED", "INTERNAL", "ABORTED":
			return true
		}
		return false
	}
	// Fallback pattern matching for SDK errors wrapped in plain strings.
	s := err.Error()
	return strings.Contains(s, "Error 429") ||
		strings.Contains(s, "RESOURCE_EXHAUSTED") ||
		strings.Contains(s, "UNAVAILABLE") ||
		strings.Contains(s, "Error 503") ||
		strings.Contains(s, "Error 500") ||
		strings.Contains(s, "Error 502") ||
		strings.Contains(s, "Error 504")
}
