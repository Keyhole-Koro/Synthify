package metering

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	joblog "github.com/synthify/backend/internal/platform/job/log"
)

// LLMClient wraps an llm.Client and ships a domain.UsageEvent through
// the provided reporter after every successful LLM call. It satisfies the
// plain llm.Client surface (and therefore tools/core/base.LLMClient) so it can be
// dropped into the existing wrapper chain.
//
// Usage attribution requires WithTag(ctx, Tag{AccountID: ...}) to have run
// upstream (worker.Process). When the tag is missing the call still succeeds
// — only the reporting step is skipped with a warn log, so a one-off tagging
// bug doesn't break the LLM pipeline.
type LLMClient struct {
	inner    llm.Client
	reporter llm.UsageReporter
	logger   *slog.Logger
}

// NewLLMClient returns a metering wrapper. reporter may be nil — passing nil
// reporter degrades the wrapper to a pass-through.
func NewLLMClient(inner llm.Client, reporter llm.UsageReporter, logger *slog.Logger) *LLMClient {
	return &LLMClient{inner: inner, reporter: reporter, logger: logger}
}

func (c *LLMClient) GenerateStructured(ctx context.Context, req llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	raw, usage, err := c.inner.GenerateStructured(ctx, req)
	if err == nil {
		c.report(ctx, usage)
	}
	return raw, usage, err
}

func (c *LLMClient) GenerateText(ctx context.Context, req llm.TextRequest) (string, llm.Usage, error) {
	text, usage, err := c.inner.GenerateText(ctx, req)
	if err == nil {
		c.report(ctx, usage)
	}
	return text, usage, err
}

func (c *LLMClient) report(ctx context.Context, usage llm.Usage) {
	reportUsage(ctx, c.reporter, c.logger, usage)
}

// reportUsage ships a single usage measurement to the billing reporter,
// attributing it to the account/workspace/job carried by the context tag.
// It is shared by the llm.Client wrapper (embedding + custom-client paths) and
// the ADK agent's after-model callback so both paths attribute usage
// identically. A nil reporter or a zero-token measurement is a no-op; a missing
// tag is logged and dropped rather than failing the call.
func reportUsage(ctx context.Context, reporter llm.UsageReporter, logger *slog.Logger, usage llm.Usage) {
	if reporter == nil {
		return
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	tag, ok := TagFromContext(ctx)
	if !ok {
		logger.Warn("metering.tag_missing",
			"model", usage.Model,
			"input_tokens", usage.InputTokens,
			"output_tokens", usage.OutputTokens,
		)
		return
	}
	event := domain.UsageEvent{
		EventID:      ulid.Make().String(),
		AccountID:    tag.AccountID,
		WorkspaceID:  tag.WorkspaceID,
		JobID:        tag.JobID,
		Model:        usage.Model,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := reporter.RecordUsage(ctx, event); err != nil {
		logger.Warn("metering.record_usage_failed",
			"error", err.Error(),
			"event_id", event.EventID,
			"account_id", event.AccountID,
			"job_id", event.JobID,
		)
		if jl := joblog.FromContext(ctx); jl != nil {
			jl.Log(ctx, joblog.Event{
				JobID:       tag.JobID,
				WorkspaceID: tag.WorkspaceID,
				Level:       joblog.WARN,
				Event:       "billing.record_usage_failed",
				Message:     "failed to ship usage event to billing API; usage may be undercounted",
				Detail: map[string]any{
					"model":         usage.Model,
					"input_tokens":  usage.InputTokens,
					"output_tokens": usage.OutputTokens,
					"error":         err.Error(),
				},
			})
		}
	}
}
