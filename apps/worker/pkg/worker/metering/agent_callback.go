package metering

import (
	"context"
	"log/slog"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
)

// NewAfterModelCallback returns an ADK after-model callback that ships token
// usage from agent LLM calls to the billing reporter. The orchestrator's agent
// path drives the model directly through ADK and therefore never crosses the
// llm.Client surface that metering.LLMClient wraps; without this callback the
// agent's generation tokens are silently dropped from billing (only the
// embedding path is metered). The callback mirrors LLMClient.report: it reuses
// the same context tag and reportUsage helper so both paths attribute usage
// identically.
//
// reporter may be nil (no-op). The ADK CallbackContext embeds context.Context,
// so the billing tag set in Worker.Process is available here.
func NewAfterModelCallback(reporter llm.UsageReporter, logger *slog.Logger) llmagent.AfterModelCallback {
	return func(ctx agent.CallbackContext, resp *model.LLMResponse, respErr error) (*model.LLMResponse, error) {
		// CallbackContext embeds context.Context, carrying the billing tag.
		reportAgentUsage(ctx, reporter, logger, resp, respErr)
		return nil, nil
	}
}

// reportAgentUsage extracts token usage from an ADK model response and ships it
// through reportUsage. It is split out so the guard conditions and field
// extraction can be tested without faking the full ADK CallbackContext.
//
// It reports nothing on errors, missing responses, or streaming partials: only
// the final non-partial event carries authoritative usage, so reporting
// partials would double-count.
func reportAgentUsage(ctx context.Context, reporter llm.UsageReporter, logger *slog.Logger, resp *model.LLMResponse, respErr error) {
	if respErr != nil || resp == nil || resp.Partial || resp.UsageMetadata == nil {
		return
	}
	reportUsage(ctx, reporter, logger, llm.Usage{
		Model:        resp.ModelVersion,
		InputTokens:  int64(resp.UsageMetadata.PromptTokenCount),
		OutputTokens: int64(resp.UsageMetadata.CandidatesTokenCount),
	})
}
