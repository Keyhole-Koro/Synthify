package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/packages/shared/domain"
)

type fakeLLM struct {
	items    []domain.GeneratedTreeItem
	rawItems bool
	usage    llm.Usage
	err      error
}

func (f fakeLLM) GenerateStructured(context.Context, llm.StructuredRequest) (json.RawMessage, llm.Usage, error) {
	if f.err != nil {
		return nil, f.usage, f.err
	}
	if f.rawItems {
		raw, err := json.Marshal(f.items)
		return raw, f.usage, err
	}
	raw, err := json.Marshal(map[string]any{"items": f.items})
	return raw, f.usage, err
}

func (f fakeLLM) GenerateText(context.Context, llm.TextRequest) (string, llm.Usage, error) {
	return "", f.usage, f.err
}

func TestRunCase_KnowledgeTreePasses(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "Authentication", Text: "Tokens expire."}})
	c := Case{
		Name:  "ok",
		Tool:  ToolKnowledgeTree,
		Input: rawInput(t, map[string]any{"document_id": "doc_1", "chunks": "chunks.json"}),
		Expect: CaseExpect{SchemaValid: true, JSON: []JSONExpect{
			{Path: "$.items", Op: "count_gte", Value: 1},
			{Path: "$.items", Op: "tree_depth_lte", Value: 2},
			{Path: "$.items[*].title", Op: "contains_all", Value: []any{"Authentication"}},
		}},
	}

	res, err := Runner{LLM: fakeLLM{
		items: []domain.GeneratedTreeItem{{
			LocalID: "item_1",
			Title:   "Authentication",
			Level:   1,
		}},
		usage: llm.Usage{Model: "fake", InputTokens: 10, OutputTokens: 20},
	}}.RunCase(context.Background(), c, dir)

	if err != nil {
		t.Fatalf("RunCase returned error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected pass, got %#v", res)
	}
	if res.Model != "fake" || res.InputTokens != 10 || res.OutputTokens != 20 {
		t.Fatalf("usage not propagated: %#v", res)
	}
	items := outputItems(t, res.Output)
	if len(items) != 1 || items[0].Title != "Authentication" {
		t.Fatalf("items not propagated in output: %#v", items)
	}
}

func TestRunCaseFiles_RunsMultipleCases(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "Authentication", Text: "Tokens expire."}})
	caseBody := []byte("name: ok\ntool: knowledge_tree\ninput:\n  document_id: doc_1\n  chunks: chunks.json\nexpect:\n  schema_valid: true\n  json:\n    - path: $.items\n      op: count_gte\n      value: 1\n")
	caseA := filepath.Join(dir, "a.yaml")
	caseB := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(caseA, caseBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caseB, caseBody, 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Runner{LLM: fakeLLM{items: []domain.GeneratedTreeItem{{
		LocalID: "item_1",
		Title:   "Authentication",
		Level:   1,
	}}}}.RunCaseFiles(context.Background(), []string{caseA, caseB})

	if err != nil {
		t.Fatalf("RunCaseFiles returned error: %v", err)
	}
	if len(results) != 2 || !results[0].Passed || !results[1].Passed {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestCaseFiles_Directory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.yml", "a.yaml", "ignore.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("name: x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := CaseFiles(dir)
	if err != nil {
		t.Fatalf("CaseFiles returned error: %v", err)
	}
	if len(files) != 2 || !strings.HasSuffix(files[0], "a.yaml") || !strings.HasSuffix(files[1], "b.yml") {
		t.Fatalf("expected sorted yaml files, got %#v", files)
	}
}

func TestRunCase_RuleFailures(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "API", Text: "Errors."}})
	c := Case{
		Name:  "bad",
		Tool:  ToolKnowledgeTree,
		Input: rawInput(t, map[string]any{"document_id": "doc_1", "chunks": "chunks.json"}),
		Expect: CaseExpect{SchemaValid: true, JSON: []JSONExpect{
			{Path: "$.items", Op: "count_gte", Value: 2},
			{Path: "$.items", Op: "tree_depth_lte", Value: 1},
			{Path: "$.items[*].title", Op: "contains_all", Value: []any{"Error Handling"}},
		}},
	}

	res, err := Runner{LLM: fakeLLM{items: []domain.GeneratedTreeItem{{
		LocalID: "item_1",
		Title:   "Authentication",
		Level:   2,
	}}}}.RunCase(context.Background(), c, dir)

	if err != nil {
		t.Fatalf("RunCase returned error: %v", err)
	}
	if res.Passed {
		t.Fatalf("expected failure, got %#v", res)
	}
	if res.FailedInput == nil || len(res.FailedInput.Chunks) != 1 || res.FailedInput.Chunks[0].Heading != "API" {
		t.Fatalf("expected failed input snapshot, got %#v", res.FailedInput)
	}
}

func TestRunCase_AcceptsTopLevelItemArray(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "認証", Text: "Tokens."}})
	c := Case{
		Name:   "array",
		Tool:   ToolKnowledgeTree,
		Input:  rawInput(t, map[string]any{"document_id": "doc_1", "chunks": "chunks.json"}),
		Expect: CaseExpect{SchemaValid: true, JSON: []JSONExpect{{Path: "$.items", Op: "count_gte", Value: 1}}},
	}

	res, err := Runner{LLM: fakeLLM{
		rawItems: true,
		items: []domain.GeneratedTreeItem{{
			LocalID: "item_1",
			Title:   "認証",
			Level:   1,
		}},
	}}.RunCase(context.Background(), c, dir)

	if err != nil {
		t.Fatalf("RunCase returned error: %v", err)
	}
	if !res.Passed || !res.SchemaValid {
		t.Fatalf("expected top-level array to pass, got %#v", res)
	}
}

func TestRunCase_ToolErrorFails(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "API", Text: "Errors."}})
	c := Case{
		Name:   "empty",
		Tool:   ToolKnowledgeTree,
		Input:  rawInput(t, map[string]any{"document_id": "doc_1", "chunks": "chunks.json"}),
		Expect: CaseExpect{SchemaValid: true, JSON: []JSONExpect{{Path: "$.items", Op: "count_gte", Value: 1}}},
	}

	res, err := Runner{LLM: fakeLLM{items: []domain.GeneratedTreeItem{}}}.RunCase(context.Background(), c, dir)
	if err != nil {
		t.Fatalf("RunCase returned error: %v", err)
	}
	if res.Passed || res.Error == "" {
		t.Fatalf("expected tool error failure, got %#v", res)
	}
}

func TestRunCase_UnsupportedTool(t *testing.T) {
	_, err := Runner{LLM: fakeLLM{}}.RunCase(context.Background(), Case{Name: "x", Tool: "summary"}, "")
	if err == nil {
		t.Fatal("expected unsupported tool error")
	}
}

func TestRunCase_PrepareValidationError(t *testing.T) {
	_, err := Runner{LLM: fakeLLM{}}.RunCase(context.Background(), Case{
		Name:   "x",
		Tool:   ToolKnowledgeTree,
		Input:  rawInput(t, map[string]any{"chunks": []any{}}),
		Expect: CaseExpect{SchemaValid: true},
	}, "")
	if err == nil {
		t.Fatal("expected missing document_id error")
	}
}

func TestRunCase_ToolLevelLLMError(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "API", Text: "Errors."}})
	c := Case{
		Name:   "llm_error",
		Tool:   ToolKnowledgeTree,
		Input:  rawInput(t, map[string]any{"document_id": "doc_1", "chunks": "chunks.json"}),
		Expect: CaseExpect{SchemaValid: true},
	}

	res, err := Runner{LLM: fakeLLM{err: errors.New("boom")}}.RunCase(context.Background(), c, dir)
	if err != nil {
		t.Fatalf("RunCase returned runner error: %v", err)
	}
	if res.Passed || res.Error != "boom" || res.FailedInput == nil {
		t.Fatalf("expected recorded tool error, got %#v", res)
	}
}

func TestRunCase_VariantPromptSource(t *testing.T) {
	dir := t.TempDir()
	variantDir := filepath.Join(dir, "concise-v1")
	if err := os.MkdirAll(variantDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(variantDir, "knowledge_tree.system.tmpl"), []byte("sys"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(variantDir, "knowledge_tree.user.tmpl"), []byte("{{.DocumentID}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "Auth", Text: "x"}})

	p, err := prompts.FromDir(variantDir)
	if err != nil {
		t.Fatalf("FromDir: %v", err)
	}

	res, err := Runner{
		LLM:          fakeLLM{items: []domain.GeneratedTreeItem{{LocalID: "item_1", Title: "Auth", Level: 1}}, usage: llm.Usage{Model: "fake"}},
		Renderer:     p,
		PromptSource: "variant:concise-v1",
	}.RunCase(context.Background(), Case{
		Name:   "variant",
		Tool:   ToolKnowledgeTree,
		Input:  rawInput(t, map[string]any{"document_id": "doc_1", "chunks": "chunks.json"}),
		Expect: CaseExpect{SchemaValid: true, JSON: []JSONExpect{{Path: "$.items", Op: "count_gte", Value: 1}}},
	}, dir)
	if err != nil {
		t.Fatalf("RunCase: %v", err)
	}
	if res.PromptSource != "variant:concise-v1" {
		t.Fatalf("expected prompt_source=variant:concise-v1, got %q", res.PromptSource)
	}
	if !res.Passed {
		t.Fatalf("expected pass with variant renderer, got %#v", res)
	}
}

func TestLoadCaseAndChunksErrors(t *testing.T) {
	dir := t.TempDir()
	badCase := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badCase, []byte("name: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCase(badCase); err == nil {
		t.Fatal("expected bad yaml error")
	}
	if _, err := LoadChunks(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("expected missing fixture error")
	}
}

func rawInput(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func outputItems(t *testing.T, raw json.RawMessage) []domain.GeneratedTreeItem {
	t.Helper()
	var out struct {
		Items []domain.GeneratedTreeItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out.Items
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
