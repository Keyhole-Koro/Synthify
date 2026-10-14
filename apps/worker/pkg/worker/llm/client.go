// Package llm provides LLM client abstractions, provider integrations, rate limiting, and token accounting for worker execution.
package llm

import (
	"context"
	"encoding/json"

	"github.com/synthify/backend/internal/domain"
)

// Client defines the core generative model contract for structured JSON and free-form text completions.
type Client interface {
	// GenerateStructured produces structured JSON conforming to a specified schema.
	GenerateStructured(ctx context.Context, req StructuredRequest) (json.RawMessage, Usage, error)
	// GenerateText produces raw text response content given system and user prompts.
	GenerateText(ctx context.Context, req TextRequest) (string, Usage, error)
}

// Usage is the per-call token accounting returned by the LLM provider.
// Model is the canonical name as the provider reports it (e.g. "gemini-3-flash-preview").
type Usage struct {
	Model        string
	InputTokens  int64
	OutputTokens int64
}

// StructuredRequest parameters for generating structured JSON output.
type StructuredRequest struct {
	SystemPrompt string
	UserPrompt   string
	SourceFiles  []domain.SourceFile
	Schema       any
}

// TextRequest parameters for generating unstructured text output.
type TextRequest struct {
	SystemPrompt string
	UserPrompt   string
	SourceFiles  []domain.SourceFile
}

// UsageReporter is the worker-side contract for shipping LLM usage to the billing API.
// The concrete implementation is a Connect client backed by the service-token auth path
// (worker → API internal call). Implementations should swallow transient errors and
// surface only invalid-input / auth failures so a metering blip never kills a job.
type UsageReporter interface {
	RecordUsage(ctx context.Context, event domain.UsageEvent) error
}
