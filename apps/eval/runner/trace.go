package runner

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/tools/core/base"
)

// TraceEvent is one persisted execution span for an eval case.
type TraceEvent struct {
	EventID      string          `json:"event_id"`
	ParentEventID string         `json:"parent_event_id,omitempty"`
	Sequence     int             `json:"sequence"`
	Kind         string          `json:"kind"`
	Name         string          `json:"name"`
	StartedAt    time.Time       `json:"started_at"`
	CompletedAt  time.Time       `json:"completed_at"`
	DurationMS   int64           `json:"duration_ms"`
	Model        string          `json:"model,omitempty"`
	InputTokens  int64           `json:"input_tokens,omitempty"`
	OutputTokens int64           `json:"output_tokens,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	Output       json.RawMessage `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
}

// TraceCollector collects events for the currently executing case. Eval cases
// are intentionally executed sequentially, so a single collector can be reused.
type TraceCollector struct {
	mu       sync.Mutex
	sequence int
	events   []TraceEvent
}

func (c *TraceCollector) BeginCase() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sequence = 0
	c.events = nil
}

func (c *TraceCollector) Record(event TraceEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sequence++
	event.Sequence = c.sequence
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	c.events = append(c.events, event)
}

func (c *TraceCollector) Events() []TraceEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]TraceEvent, len(c.events))
	copy(out, c.events)
	return out
}

type tracingLLM struct {
	next      base.LLMClient
	collector *TraceCollector
}

func NewTracingLLM(next base.LLMClient, collector *TraceCollector) base.LLMClient {
	return &tracingLLM{next: next, collector: collector}
}

func (t *tracingLLM) GenerateStructured(ctx context.Context, req llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	started := time.Now().UTC()
	out, usage, err := t.next.GenerateStructured(ctx, req)
	completed := time.Now().UTC()
	input, _ := json.Marshal(map[string]any{
		"system_prompt": req.SystemPrompt,
		"user_prompt": req.UserPrompt,
		"source_files": req.SourceFiles,
		"schema": req.Schema,
	})
	event := TraceEvent{
		Kind: "llm", Name: "GenerateStructured", StartedAt: started, CompletedAt: completed,
		DurationMS: completed.Sub(started).Milliseconds(), Model: usage.Model,
		InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
		Input: input, Output: out,
	}
	if err != nil {
		event.Error = err.Error()
	}
	t.collector.Record(event)
	return out, usage, err
}

func (t *tracingLLM) GenerateText(ctx context.Context, req llm.TextRequest) (string, llm.Usage, error) {
	started := time.Now().UTC()
	out, usage, err := t.next.GenerateText(ctx, req)
	completed := time.Now().UTC()
	input, _ := json.Marshal(map[string]any{
		"system_prompt": req.SystemPrompt,
		"user_prompt": req.UserPrompt,
		"source_files": req.SourceFiles,
	})
	output, _ := json.Marshal(out)
	event := TraceEvent{
		Kind: "llm", Name: "GenerateText", StartedAt: started, CompletedAt: completed,
		DurationMS: completed.Sub(started).Milliseconds(), Model: usage.Model,
		InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
		Input: input, Output: output,
	}
	if err != nil {
		event.Error = err.Error()
	}
	t.collector.Record(event)
	return out, usage, err
}
