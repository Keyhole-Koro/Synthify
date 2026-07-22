package runner

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/domain"
	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
)

// TestRunCase_CollectsTrace proves the Collector wiring captures a real LLM span
// (from the tracing wrapper around the tool's model call) plus the validation
// and assertion spans, ordered, for a single case.
func TestRunCase_CollectsTrace(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "Auth", Text: "x"}})

	c := Case{
		Name:   "traced",
		Tool:   ToolKnowledgeTree,
		Input:  rawInput(t, map[string]any{"document_id": "doc_1", "chunks": "chunks.json"}),
		Expect: CaseExpect{SchemaValid: true, JSON: []JSONExpect{{Path: "$.items", Op: "count_gte", Value: 1}}},
	}

	res, err := Runner{
		LLM:       fakeLLM{items: []domain.GeneratedTreeItem{{LocalID: "i1", Title: "Auth", Level: 1}}, usage: llm.Usage{Model: "fake", InputTokens: 3, OutputTokens: 4}},
		Collector: &TraceCollector{},
	}.RunCase(context.Background(), c, dir)
	if err != nil {
		t.Fatalf("RunCase: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected pass, got %#v", res)
	}

	kinds := make([]string, 0, len(res.Trace))
	for i, ev := range res.Trace {
		if ev.Sequence != i+1 {
			t.Errorf("event %d out of order: sequence=%d", i, ev.Sequence)
		}
		if ev.EventID == "" {
			t.Errorf("event %d missing event_id", i)
		}
		kinds = append(kinds, ev.Kind)
	}
	if len(kinds) != 4 || kinds[0] != "tool" || kinds[1] != "llm" || kinds[2] != "validation" || kinds[3] != "assertion" {
		t.Fatalf("trace kinds = %v, want [tool llm validation assertion]", kinds)
	}

	toolEv := res.Trace[0]
	llmEv := res.Trace[1]
	// The llm call must nest under the tool span; validation/assertion are
	// top-level siblings recorded after the tool returned.
	if llmEv.ParentEventID != toolEv.EventID {
		t.Errorf("llm span parent = %q, want tool span %q", llmEv.ParentEventID, toolEv.EventID)
	}
	if res.Trace[2].ParentEventID != "" || res.Trace[3].ParentEventID != "" {
		t.Errorf("validation/assertion should be top-level, got parents %q / %q", res.Trace[2].ParentEventID, res.Trace[3].ParentEventID)
	}
	if toolEv.ParentEventID != "" {
		t.Errorf("tool span should be top-level, got parent %q", toolEv.ParentEventID)
	}
	if toolEv.DurationMS < 0 {
		t.Errorf("tool span duration not finalized: %#v", toolEv)
	}
	if llmEv.Model != "fake" || llmEv.InputTokens != 3 || llmEv.OutputTokens != 4 {
		t.Errorf("llm span usage not recorded: %#v", llmEv)
	}
	if len(llmEv.Input) == 0 {
		t.Error("llm span input (prompt) not recorded")
	}
	var promptFields map[string]any
	if err := json.Unmarshal(llmEv.Input, &promptFields); err != nil {
		t.Fatalf("llm input is not JSON: %v", err)
	}
	if _, ok := promptFields["user_prompt"]; !ok {
		t.Errorf("llm input missing user_prompt: %v", promptFields)
	}
}

// TestRunCase_NoCollectorNoTrace confirms trace stays empty (and thus omitted
// from the report) when no collector is attached.
func TestRunCase_NoCollectorNoTrace(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "Auth", Text: "x"}})
	res, err := Runner{LLM: fakeLLM{items: []domain.GeneratedTreeItem{{LocalID: "i1", Title: "Auth", Level: 1}}}}.RunCase(
		context.Background(),
		Case{Name: "x", Tool: ToolKnowledgeTree, Input: rawInput(t, map[string]any{"document_id": "d", "chunks": "chunks.json"}), Expect: CaseExpect{SchemaValid: true}},
		dir,
	)
	if err != nil {
		t.Fatalf("RunCase: %v", err)
	}
	if len(res.Trace) != 0 {
		t.Fatalf("expected no trace without collector, got %d events", len(res.Trace))
	}
}
