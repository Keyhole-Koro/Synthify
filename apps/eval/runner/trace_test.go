package runner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
)

type traceFakeLLM struct{}

func (traceFakeLLM) GenerateStructured(context.Context, llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	return json.RawMessage(`{"ok":true}`), llm.Usage{Model: "fake-model", InputTokens: 11, OutputTokens: 7}, nil
}

func (traceFakeLLM) GenerateText(context.Context, llm.TextRequest) (string, llm.Usage, error) {
	return "hello", llm.Usage{Model: "fake-model", InputTokens: 3, OutputTokens: 2}, nil
}

func TestTracingLLMRecordsStructuredCall(t *testing.T) {
	collector := &TraceCollector{}
	collector.BeginCase()
	client := NewTracingLLM(traceFakeLLM{}, collector)

	_, _, err := client.GenerateStructured(context.Background(), llm.StructuredRequest{SystemPrompt: "system", UserPrompt: "user"})
	if err != nil {
		t.Fatal(err)
	}
	events := collector.Events()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Kind != "llm" || events[0].Name != "GenerateStructured" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
	if events[0].Model != "fake-model" || events[0].InputTokens != 11 || events[0].OutputTokens != 7 {
		t.Fatalf("unexpected usage: %+v", events[0])
	}
}
