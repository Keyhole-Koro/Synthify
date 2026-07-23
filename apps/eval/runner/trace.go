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
	EventID       string          `json:"event_id"`
	ParentEventID string          `json:"parent_event_id,omitempty"`
	Sequence      int             `json:"sequence"`
	Kind          string          `json:"kind"`
	Name          string          `json:"name"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   time.Time       `json:"completed_at"`
	DurationMS    int64           `json:"duration_ms"`
	Model         string          `json:"model,omitempty"`
	InputTokens   int64           `json:"input_tokens,omitempty"`
	OutputTokens  int64           `json:"output_tokens,omitempty"`
	Input         json.RawMessage `json:"input,omitempty"`
	Output        json.RawMessage `json:"output,omitempty"`
	Error         string          `json:"error,omitempty"`
}

// TraceCollector collects events for the currently executing case. Eval cases
// are intentionally executed sequentially, so a single collector can be reused.
//
// It maintains a span stack so nested work is recorded with the right parent:
// a tool span is opened before the tool runs, and every LLM call the tool makes
// while that span is open is recorded as its child (parent_event_id). This is
// what turns the flat llm/validation/assertion list into a real tool → llm tree.
type TraceCollector struct {
	mu       sync.Mutex
	sequence int
	events   []TraceEvent
	stack    []string // open parent span IDs, innermost last
}

func (c *TraceCollector) BeginCase() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sequence = 0
	c.events = nil
	c.stack = nil
}

func (c *TraceCollector) Record(event TraceEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordLocked(event)
}

// recordLocked appends an event, assigning its sequence, id, and — unless the
// caller set one explicitly — the innermost open span as its parent. Callers
// must hold c.mu.
func (c *TraceCollector) recordLocked(event TraceEvent) string {
	c.sequence++
	event.Sequence = c.sequence
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	if event.ParentEventID == "" && len(c.stack) > 0 {
		event.ParentEventID = c.stack[len(c.stack)-1]
	}
	c.events = append(c.events, event)
	return event.EventID
}

// BeginSpan records an open parent span (e.g. a tool invocation) and pushes it
// onto the stack so subsequent events nest under it. The span is recorded
// immediately — before its children — so the ordered sequence reads parent-first.
// Pair every BeginSpan with EndSpan(id, ...).
func (c *TraceCollector) BeginSpan(kind, name string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.recordLocked(TraceEvent{Kind: kind, Name: name, StartedAt: time.Now().UTC()})
	c.stack = append(c.stack, id)
	return id
}

// EndSpan finalizes an open span: it pops the stack and fills in the span's
// completion time, duration (from its recorded StartedAt) and error.
func (c *TraceCollector) EndSpan(id, errStr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.stack) - 1; i >= 0; i-- {
		if c.stack[i] == id {
			c.stack = append(c.stack[:i], c.stack[i+1:]...)
			break
		}
	}
	now := time.Now().UTC()
	for i := range c.events {
		if c.events[i].EventID == id {
			c.events[i].CompletedAt = now
			c.events[i].DurationMS = now.Sub(c.events[i].StartedAt).Milliseconds()
			if errStr != "" {
				c.events[i].Error = errStr
			}
			return
		}
	}
}

// RecordValidation records the schema-validation span for the current case.
// Unlike LLM spans it is instantaneous (the check runs in-process), so start and
// end coincide; the value is the ordered pass/fail record, not a duration.
func (c *TraceCollector) RecordValidation(schemaValid bool, caseError string) {
	now := time.Now().UTC()
	ev := TraceEvent{Kind: "validation", Name: "schema_validation", StartedAt: now, CompletedAt: now}
	if !schemaValid {
		ev.Error = firstNonEmpty(caseError, "output failed schema validation")
	}
	c.Record(ev)
}

// RecordAssertion records the semantic-assertion span (expect.json rules) for
// the current case.
func (c *TraceCollector) RecordAssertion(passed bool, caseError string) {
	now := time.Now().UTC()
	ev := TraceEvent{Kind: "assertion", Name: "semantic_assertions", StartedAt: now, CompletedAt: now}
	if !passed {
		ev.Error = firstNonEmpty(caseError, "expectation failed")
	}
	c.Record(ev)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
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
		"user_prompt":   req.UserPrompt,
		"source_files":  req.SourceFiles,
		"schema":        req.Schema,
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
		"user_prompt":   req.UserPrompt,
		"source_files":  req.SourceFiles,
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
