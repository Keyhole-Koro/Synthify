package base

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	joblog "github.com/synthify/backend/internal/platform/job/log"
)

type sessionIDContext interface {
	SessionID() string
}

type usageCounters struct {
	capability         *domain.JobCapability
	llmCalls           int
	toolRuns           int
	itemCreations      int
	transformCreations int
	transformRuns      int
}

// UsageLimiter tracks per-job worker resource usage and enforces JobCapability limits.
type UsageLimiter struct {
	repo   Repository
	logger *slog.Logger

	mu   sync.Mutex
	jobs map[string]*usageCounters
}

func NewUsageLimiter(repo Repository, logger *slog.Logger) *UsageLimiter {
	return &UsageLimiter{
		repo:   repo,
		logger: logger,
		jobs:   make(map[string]*usageCounters),
	}
}

func (l *UsageLimiter) BeginJob(ctx context.Context, jobID string) {
	if l == nil || jobID == "" {
		return
	}
	capability, _ := l.loadCapability(ctx, jobID)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.jobs[jobID] = &usageCounters{capability: capability}
}

func (l *UsageLimiter) IncrementLLMCalls(ctx context.Context) error {
	return l.increment(ctx, 1, 0, 0, 0, 0)
}

func (l *UsageLimiter) IncrementToolRuns(ctx context.Context) error {
	return l.increment(ctx, 0, 1, 0, 0, 0)
}

func (l *UsageLimiter) IncrementItemCreations(ctx context.Context, count int) error {
	if count <= 0 {
		return nil
	}
	return l.increment(ctx, 0, 0, count, 0, 0)
}

// IncrementTransformCreations counts a create_transform invocation. The same
// counter is consumed by syntax_error fix-regeneration so a malformed-code
// loop cannot run unbounded (see design doc "コストと再帰の暴走防止").
func (l *UsageLimiter) IncrementTransformCreations(ctx context.Context) error {
	return l.increment(ctx, 0, 0, 0, 1, 0)
}

// IncrementTransformRuns counts a dynamic-tool execution (ephemeral or
// promoted), capped separately from MaxToolRuns.
func (l *UsageLimiter) IncrementTransformRuns(ctx context.Context) error {
	return l.increment(ctx, 0, 0, 0, 0, 1)
}

func (l *UsageLimiter) increment(ctx context.Context, llmCalls, toolRuns, itemCreations, transformCreations, transformRuns int) error {
	if l == nil {
		return nil
	}
	jobID := jobIDFromContext(ctx)
	if jobID == "" {
		return nil
	}

	l.mu.Lock()
	counters := l.jobs[jobID]
	l.mu.Unlock()

	if counters == nil {
		capability, _ := l.loadCapability(ctx, jobID)
		l.mu.Lock()
		counters = l.jobs[jobID]
		if counters == nil {
			counters = &usageCounters{capability: capability}
			l.jobs[jobID] = counters
		}
		l.mu.Unlock()
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	counters.llmCalls += llmCalls
	counters.toolRuns += toolRuns
	counters.itemCreations += itemCreations
	counters.transformCreations += transformCreations
	counters.transformRuns += transformRuns
	capability := counters.capability

	// Surface the running model-call count for every model invocation. This is
	// the only place the count lives (it drives MaxLLMCalls enforcement), and
	// it sits on the ADK agent path — the path that actually drives Vertex — so
	// it answers "how many calls did this job make before the 429?", which the
	// success-only GeminiClient logging could not.
	if llmCalls > 0 {
		maxLLM := 0
		if capability != nil {
			maxLLM = capability.MaxLLMCalls
		}
		joblog.FromContext(ctx).Log(ctx, joblog.Event{
			JobID:   jobID,
			Level:   joblog.INFO,
			Event:   "llm.call",
			Message: fmt.Sprintf("model call #%d (max %d)", counters.llmCalls, maxLLM),
			Detail:  map[string]any{"llm_calls": counters.llmCalls, "max_llm_calls": maxLLM},
		})
	}

	if capability == nil {
		return nil
	}

	for _, lim := range []struct {
		resource string
		count    int
		max      int
	}{
		{"llm_calls", counters.llmCalls, capability.MaxLLMCalls},
		{"tool_runs", counters.toolRuns, capability.MaxToolRuns},
		{"item_creations", counters.itemCreations, capability.MaxItemCreations},
		{"transform_creations", counters.transformCreations, capability.MaxTransformCreations},
		{"transform_runs", counters.transformRuns, capability.MaxTransformRuns},
	} {
		if lim.max > 0 && lim.count > lim.max {
			return l.reportExceeded(ctx, jobID, lim.resource, lim.count, lim.max)
		}
	}
	return nil
}

func (l *UsageLimiter) reportExceeded(ctx context.Context, jobID, resource string, count, max int) error {
	err := fmt.Errorf("job %s exceeded %s limit: %d > %d", jobID, resource, count, max)
	l.logger.Error("usage.limit_exceeded", "error", err.Error(), "job_id", jobID, "resource", resource, "count", count, "limit", max)
	joblog.FromContext(ctx).Log(ctx, joblog.Event{
		JobID:   jobID,
		Level:   joblog.ERROR,
		Event:   "llm.usage_exceeded",
		Message: fmt.Sprintf("%s limit exceeded: %d/%d", resource, count, max),
		Detail:  map[string]any{"resource": resource, "count": count, "limit": max},
	})
	return err
}

func (l *UsageLimiter) loadCapability(ctx context.Context, jobID string) (*domain.JobCapability, error) {
	if l == nil || l.repo == nil || jobID == "" {
		return nil, domain.ErrNotFound
	}
	return l.repo.GetJobCapability(ctx, jobID)
}

func jobIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if sessionCtx, ok := ctx.(sessionIDContext); ok {
		return sessionCtx.SessionID()
	}
	return ""
}

// CountingLLMClient counts direct tool-level LLM calls before delegating to the real client.
type CountingLLMClient struct {
	inner LLMClient
	usage *UsageLimiter
}

func NewCountingLLMClient(inner LLMClient, usage *UsageLimiter) LLMClient {
	if inner == nil {
		return nil
	}
	return &CountingLLMClient{inner: inner, usage: usage}
}

func (c *CountingLLMClient) GenerateStructured(ctx context.Context, req llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	if c != nil && c.usage != nil {
		if err := c.usage.IncrementLLMCalls(ctx); err != nil {
			return nil, llm.Usage{}, err
		}
	}
	return c.inner.GenerateStructured(ctx, req)
}

func (c *CountingLLMClient) GenerateText(ctx context.Context, req llm.TextRequest) (string, llm.Usage, error) {
	if c != nil && c.usage != nil {
		if err := c.usage.IncrementLLMCalls(ctx); err != nil {
			return "", llm.Usage{}, err
		}
	}
	return c.inner.GenerateText(ctx, req)
}
