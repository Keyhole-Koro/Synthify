package llm

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"math/rand/v2"
	"strings"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// RetryConfig は ADK model.LLM ラッパーの再試行ポリシー。
type RetryConfig struct {
	MaxAttempts int           // 初回試行を含む合計回数
	BaseDelay   time.Duration // 指数バックオフの基準
	MaxDelay    time.Duration // バックオフの上限
	Jitter      float64       // [0,1) の比率でジッタを乗せる
}

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

// RetryingModel は 429 / 5xx / UNAVAILABLE を捕捉して指数バックオフで
// 再試行する model.LLM ラッパー。successful response がひとつでも
// yield された後の error は再試行せずそのまま通す。
//
// 加えて、クライアント側の流量制御として model 固有の RPM (QuotaFor) を
// 上限とする rate limiter を持つ。これが無いと agent ループが連続で Vertex
// を叩き、preview 帯の極小 RPM を即超過して 429 が多発する。limiter で
// 呼び出しを平滑化し、リトライ/バックオフはあくまで保険として残す。
type RetryingModel struct {
	inner   model.LLM
	cfg     RetryConfig
	limiter *rate.Limiter
	logger  *slog.Logger
}

func NewRetryingModel(inner model.LLM, cfg RetryConfig, logger *slog.Logger) *RetryingModel {
	if logger == nil {
		logger = slog.Default()
	}
	return &RetryingModel{
		inner:   inner,
		cfg:     cfg.withDefaults(),
		limiter: newRateLimiter(inner.Name()),
		logger:  logger,
	}
}

// newRateLimiter builds a per-minute limiter from the model's reference RPM.
// We allow a small burst (up to the full minute's budget, capped) so short
// bursts are not serialized to a crawl, while the steady-state rate stays
// under the quota.
func newRateLimiter(model string) *rate.Limiter {
	rpm := QuotaFor(model).RPM
	if rpm <= 0 {
		rpm = defaultQuota.RPM
	}
	perSecond := rate.Limit(float64(rpm) / 60.0)
	burst := rpm
	if burst > 10 {
		burst = 10
	}
	if burst < 1 {
		burst = 1
	}
	return rate.NewLimiter(perSecond, burst)
}

func (m *RetryingModel) Name() string { return m.inner.Name() }

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
				// 部分応答後の失敗は安全に再試行できない。
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
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			}
		}
	}
}

// backoff は attempt(1-origin) に対して指数バックオフ + ジッタを返す。
// 429 の Retry-After 系情報は genai.APIError.Details に乗ることがあるが、
// 形式が安定しないので現状は無視して指数バックオフのみ。
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

// isRetryable は genai.APIError や ADK が wrap した err 文字列から
// 再試行可能カテゴリを判定する。
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
	// genai が APIError を返さず文字列でラップしているケースのフォールバック。
	s := err.Error()
	return strings.Contains(s, "Error 429") ||
		strings.Contains(s, "RESOURCE_EXHAUSTED") ||
		strings.Contains(s, "UNAVAILABLE") ||
		strings.Contains(s, "Error 503") ||
		strings.Contains(s, "Error 500") ||
		strings.Contains(s, "Error 502") ||
		strings.Contains(s, "Error 504")
}
