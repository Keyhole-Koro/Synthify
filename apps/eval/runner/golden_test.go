package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/synthify/backend/apps/worker/pkg/worker/llm"
	"github.com/synthify/backend/apps/worker/pkg/worker/prompts"
	"github.com/synthify/backend/packages/shared/domain"
)

func goldenCase() Case {
	return Case{
		Name:   "g",
		Tool:   ToolSynthesis,
		Input:  CaseInput{DocumentID: "doc_1", Chunks: "chunks.json"},
		Expect: CaseExpect{SchemaValid: true, MinItems: 1},
	}
}

func goldenItems() []domain.SynthesizedItem {
	return []domain.SynthesizedItem{
		{
			LocalID:        "item_1",
			Title:          "Auth",
			Level:          1,
			Content:        "<p class=\"lede\">huge html body that must never leak into the diff</p>",
			SourceChunkIDs: []string{"doc_1_chunk_0"},
		},
	}
}

func TestGolden_UpdateThenMatch(t *testing.T) {
	dir := t.TempDir()
	goldenDir := filepath.Join(dir, "golden")
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "Auth", Text: "x"}})

	llmStub := fakeLLM{items: goldenItems()}

	// 1. --update-golden writes the strict snapshot.
	_, err := Runner{LLM: llmStub, GoldenDir: goldenDir, UpdateGolden: true}.
		RunCase(context.Background(), goldenCase(), dir)
	if err != nil {
		t.Fatalf("update-golden RunCase: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(goldenDir, "g.json"))
	if err != nil {
		t.Fatalf("golden not written: %v", err)
	}
	if strings.Contains(string(raw), "huge html body") {
		t.Fatalf("golden file leaked full content HTML: %s", raw)
	}

	// 2. --golden against the same output matches.
	res, err := Runner{LLM: llmStub, GoldenDir: goldenDir}.
		RunCase(context.Background(), goldenCase(), dir)
	if err != nil {
		t.Fatalf("golden judge RunCase: %v", err)
	}
	if !res.GoldenChecked || !res.GoldenMatch || !res.Passed {
		t.Fatalf("expected golden match + pass, got %#v", res)
	}
	if res.PromptSource != "production" {
		t.Fatalf("expected prompt_source=production, got %q", res.PromptSource)
	}
}

func TestGolden_MismatchFailsAndOmitsContent(t *testing.T) {
	dir := t.TempDir()
	goldenDir := filepath.Join(dir, "golden")
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "Auth", Text: "x"}})

	if _, err := (Runner{LLM: fakeLLM{items: goldenItems()}, GoldenDir: goldenDir, UpdateGolden: true}.
		RunCase(context.Background(), goldenCase(), dir)); err != nil {
		t.Fatalf("seed golden: %v", err)
	}

	// Different title set + extra item: strict mismatch.
	drifted := []domain.SynthesizedItem{
		{LocalID: "item_1", Title: "Renamed", Level: 1, Content: "<p>secret content</p>", SourceChunkIDs: []string{"doc_1_chunk_0"}},
		{LocalID: "item_2", Title: "Extra", Level: 1},
	}
	res, err := Runner{LLM: fakeLLM{items: drifted}, GoldenDir: goldenDir}.
		RunCase(context.Background(), goldenCase(), dir)
	if err != nil {
		t.Fatalf("golden judge RunCase: %v", err)
	}
	if res.Passed || res.GoldenMatch {
		t.Fatalf("expected golden mismatch failure, got %#v", res)
	}
	if res.GoldenDiff == nil || len(res.GoldenDiff.Fields) == 0 {
		t.Fatalf("expected populated golden diff, got %#v", res.GoldenDiff)
	}
	for _, f := range res.GoldenDiff.Fields {
		if strings.Contains(f.Expected, "secret content") || strings.Contains(f.Actual, "secret content") ||
			strings.Contains(f.Expected, "<p>") || strings.Contains(f.Actual, "<p>") {
			t.Fatalf("golden diff leaked content HTML in field %q: %#v", f.Field, f)
		}
	}
}

func TestRunCase_VariantPromptSource(t *testing.T) {
	dir := t.TempDir()
	variantDir := filepath.Join(dir, "concise-v1")
	if err := os.MkdirAll(variantDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Minimal valid variant templates.
	if err := os.WriteFile(filepath.Join(variantDir, "synthesis.system.tmpl"), []byte("sys"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(variantDir, "synthesis.user.tmpl"), []byte("{{.DocumentID}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "chunks.json"), []domain.Chunk{{ChunkIndex: 0, Heading: "Auth", Text: "x"}})

	p, err := prompts.FromDir(variantDir)
	if err != nil {
		t.Fatalf("FromDir: %v", err)
	}

	res, err := Runner{
		LLM:          fakeLLM{items: goldenItems(), usage: llm.Usage{Model: "fake"}},
		Provider:     p,
		PromptSource: "variant:concise-v1",
	}.RunCase(context.Background(), goldenCase(), dir)
	if err != nil {
		t.Fatalf("RunCase: %v", err)
	}
	if res.PromptSource != "variant:concise-v1" {
		t.Fatalf("expected prompt_source=variant:concise-v1, got %q", res.PromptSource)
	}
	if !res.Passed {
		t.Fatalf("expected pass with variant provider, got %#v", res)
	}
}
